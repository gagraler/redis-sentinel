/*
 *
 * Copyright 2023 keington.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package utils

import (
	"context"
	"fmt"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"path"
	v1 "redis-sentinel/api/v1"
	"sort"
	"strconv"
	"strings"
)

const (
	redisExporterContainer = "redis-exporter"
)

// statefulSetParameters will define stateFulSets input params
type statefulSetParameters struct {
	Replicas                      *int32
	ClusterMode                   bool
	Metadata                      metav1.ObjectMeta
	NodeSelector                  map[string]string
	PodSecurityContext            *corev1.PodSecurityContext
	PriorityClassName             string
	Affinity                      *corev1.Affinity
	Tolerations                   *[]corev1.Toleration
	EnableMetrics                 bool
	PersistentVolumeClaim         corev1.PersistentVolumeClaim
	NodeConfPersistentVolumeClaim corev1.PersistentVolumeClaim
	ImagePullSecrets              *[]corev1.LocalObjectReference
	ExternalConfig                *string
	ServiceAccountName            *string
	UpdateStrategy                appsv1.StatefulSetUpdateStrategy
	RecreateStatefulSet           bool
	InitContainers                *[]v1.InitContainer
	TerminationGracePeriodSeconds *int64
}

// containerParameters will define container input params
type containerParameters struct {
	Image                        string
	ImagePullPolicy              corev1.PullPolicy
	Resources                    *corev1.ResourceRequirements
	SecurityContext              *corev1.SecurityContext
	RedisExporterImage           string
	RedisExporterImagePullPolicy corev1.PullPolicy
	RedisExporterResources       *corev1.ResourceRequirements
	RedisExporterEnv             *[]corev1.EnvVar
	Role                         string
	EnabledPassword              *bool
	SecretName                   *string
	SecretKey                    *string
	PersistenceEnabled           *bool
	TLSConfig                    *v1.TLSConfig
	ACLConfig                    *v1.ACLConfig
	ReadinessProbe               *v1.Probe
	LivenessProbe                *v1.Probe
	AdditionalEnvVariable        *[]corev1.EnvVar
	AdditionalVolume             []corev1.Volume
	AdditionalMountPath          []corev1.VolumeMount
}

type initContainerParameters struct {
	Enabled               *bool
	Image                 string
	ImagePullPolicy       corev1.PullPolicy
	Resources             *corev1.ResourceRequirements
	Role                  string
	Command               []string
	Arguments             []string
	PersistenceEnabled    *bool
	AdditionalEnvVariable *[]corev1.EnvVar
	AdditionalVolume      []corev1.Volume
	AdditionalMountPath   []corev1.VolumeMount
}

// CreateOrUpdateStateFul method will create or update Redis service
func CreateOrUpdateStateFul(namespace string, stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initContainerParams initContainerParameters, containerParams containerParameters, sidecars *[]v1.Sidecar) error {
	logger := stateFulSetLogger(namespace, stsMeta.Name)
	storedStateful, err := GetStatefulSet(namespace, stsMeta.Name)
	statefulSetDef := generateStatefulSetsDef(stsMeta, params, ownerDef, initContainerParams, containerParams, getSidecars(sidecars))
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil {
			logger.Error(err, "Unable to patch redis stateFulSet with comparison object")
			return err
		}
		if apierror.IsNotFound(err) {
			return createStateFulSet(namespace, statefulSetDef)
		}
		return err
	}
	return patchStatefulSet(storedStateful, statefulSetDef, namespace, params.RecreateStatefulSet)
}

// patchStateFulSet will patch Redis Kubernetes StateFulSet
func patchStatefulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string, recreateStateFulSet bool) error {
	logger := stateFulSetLogger(namespace, storedStateful.Name)

	// We want to try and keep this atomic as possible.
	newStateful.ResourceVersion = storedStateful.ResourceVersion
	newStateful.CreationTimestamp = storedStateful.CreationTimestamp
	newStateful.ManagedFields = storedStateful.ManagedFields

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful,
		patch.IgnoreStatusFields(),
		patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(),
		patch.IgnoreField("kind"),
		patch.IgnoreField("apiVersion"),
	)
	if err != nil {
		logger.Error(err, "Unable to patch redis stateFulSet with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		logger.Info("Changes in stateFulSet Detected, Updating...", "patch", string(patchResult.Patch))
		if len(newStateful.Spec.VolumeClaimTemplates) >= 1 && len(newStateful.Spec.VolumeClaimTemplates) == len(storedStateful.Spec.VolumeClaimTemplates) {
			// Field is immutable therefore we MUST keep it as is.
			if !apiequality.Semantic.DeepEqual(newStateful.Spec.VolumeClaimTemplates[0].Spec, storedStateful.Spec.VolumeClaimTemplates[0].Spec) {
				// resize pvc
				// 1.Get the data already stored internally
				// 2.Get the desired data
				// 3.Start querying the pvc list when you find data inconsistencies
				// 3.1 Comparison using real pvc capacity and desired data
				// 3.1.1 Update if you find inconsistencies
				// 3.2 Writing successful updates to internal
				// 4. Set to old VolumeClaimTemplates to update.Prevent update error reporting
				// 5. Set to old annotations to update
				annotations := storedStateful.Annotations
				if annotations == nil {
					annotations = map[string]string{
						"storageCapacity": "0",
					}
				}
				storedCapacity, _ := strconv.ParseInt(annotations["storageCapacity"], 0, 64)
				if len(newStateful.Spec.VolumeClaimTemplates) != 0 {
					stateCapacity := newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()
					if storedCapacity != stateCapacity {
						listOpt := metav1.ListOptions{
							LabelSelector: labels.FormatLabels(
								map[string]string{
									"app":                         storedStateful.Name,
									"app.kubernetes.io/component": "redis",
									"app.kubernetes.io/name":      storedStateful.Name,
								},
							),
						}
						pvcs, err := createKubernetesClient().CoreV1().PersistentVolumeClaims(storedStateful.Namespace).List(context.Background(), listOpt)
						if err != nil {
							return err
						}
						updateFailed := false
						realUpdate := false
						for _, pvc := range pvcs.Items {
							realCapacity := pvc.Spec.Resources.Requests.Storage().Value()
							if realCapacity != stateCapacity {
								realUpdate = true
								pvc.Spec.Resources.Requests = newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests
								_, err = createKubernetesClient().CoreV1().PersistentVolumeClaims(storedStateful.Namespace).Update(context.Background(), &pvc, metav1.UpdateOptions{})
								if err != nil {
									if !updateFailed {
										updateFailed = true
									}
									logger.Error(fmt.Errorf("redis:%s resize pvc failed:%s", storedStateful.Name, err.Error()), "")
								}
							}
						}
						if !updateFailed && len(pvcs.Items) != 0 {
							annotations["storageCapacity"] = fmt.Sprintf("%d", stateCapacity)
							storedStateful.Annotations = annotations
							if realUpdate {
								logger.Info(fmt.Sprintf("redis:%s resize pvc from  %d to %d", storedStateful.Name, storedCapacity, stateCapacity))
							} else {
								logger.Info(fmt.Sprintf("redis:%s resize noting,just set annotations", storedStateful.Name))
							}
						}
					}
				}
			}
			newStateful.Annotations["storageCapacity"] = storedStateful.Annotations["storageCapacity"]
			// set stored.volumeClaimTemplates
			newStateful.Spec.VolumeClaimTemplates = storedStateful.Spec.VolumeClaimTemplates
		}

		for key, value := range storedStateful.Annotations {
			if _, present := newStateful.Annotations[key]; !present {
				newStateful.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
			logger.Error(err, "Unable to patch redis stateFulSet with comparison object")
			return err
		}
		return updateStatefulSet(namespace, newStateful, recreateStateFulSet)
	}
	logger.Info("Reconciliation Complete, no Changes required.")
	return nil
}

// generateStatefulSetsDef generates the stateFulSets definition of Redis
func generateStatefulSetsDef(stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initContainerParams initContainerParameters, containerParams containerParameters, sidecars []v1.Sidecar) *appsv1.StatefulSet {
	stateFulSet := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:       LabelSelectors(stsMeta.GetLabels()),
			ServiceName:    fmt.Sprintf("%s-headless", stsMeta.Name),
			Replicas:       params.Replicas,
			UpdateStrategy: params.UpdateStrategy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      stsMeta.GetLabels(),
					Annotations: generateStatefulSetsAnots(stsMeta),
				},
				Spec: corev1.PodSpec{
					Containers: generateContainerDef(
						stsMeta.GetName(),
						containerParams,
						params.ClusterMode,
						params.EnableMetrics,
						params.ExternalConfig,
						containerParams.AdditionalMountPath,
						sidecars,
					),
					NodeSelector:                  params.NodeSelector,
					SecurityContext:               params.PodSecurityContext,
					PriorityClassName:             params.PriorityClassName,
					Affinity:                      params.Affinity,
					TerminationGracePeriodSeconds: params.TerminationGracePeriodSeconds,
				},
			},
		},
	}

	if initContainerParams.Enabled != nil && *initContainerParams.Enabled {
		stateFulSet.Spec.Template.Spec.InitContainers = generateInitContainerDef(stsMeta.GetName(), initContainerParams, initContainerParams.AdditionalMountPath)
	}
	if params.Tolerations != nil {
		stateFulSet.Spec.Template.Spec.Tolerations = *params.Tolerations
	}
	if params.ImagePullSecrets != nil {
		stateFulSet.Spec.Template.Spec.ImagePullSecrets = *params.ImagePullSecrets
	}
	if containerParams.PersistenceEnabled != nil && params.ClusterMode {
		stateFulSet.Spec.VolumeClaimTemplates = append(stateFulSet.Spec.VolumeClaimTemplates, createPVCTemplate("node-conf", stsMeta, params.NodeConfPersistentVolumeClaim))
	}
	if containerParams.PersistenceEnabled != nil && *containerParams.PersistenceEnabled {
		stateFulSet.Spec.VolumeClaimTemplates = append(stateFulSet.Spec.VolumeClaimTemplates, createPVCTemplate(stsMeta.GetName(), stsMeta, params.PersistentVolumeClaim))
	}
	if params.ExternalConfig != nil {
		stateFulSet.Spec.Template.Spec.Volumes = getExternalConfig(*params.ExternalConfig)
	}
	if containerParams.AdditionalVolume != nil {
		stateFulSet.Spec.Template.Spec.Volumes = append(stateFulSet.Spec.Template.Spec.Volumes, containerParams.AdditionalVolume...)
	}

	if containerParams.TLSConfig != nil {
		stateFulSet.Spec.Template.Spec.Volumes = append(stateFulSet.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &containerParams.TLSConfig.Secret,
				},
			})
	}

	if containerParams.ACLConfig != nil && containerParams.ACLConfig.Secret != nil {
		stateFulSet.Spec.Template.Spec.Volumes = append(stateFulSet.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "acl-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: containerParams.ACLConfig.Secret,
				},
			})
	}

	if params.ServiceAccountName != nil {
		stateFulSet.Spec.Template.Spec.ServiceAccountName = *params.ServiceAccountName
	}

	AddOwnerRefToObject(stateFulSet, ownerDef)
	return stateFulSet
}

// getExternalConfig will return the redis external configuration
func getExternalConfig(configMapName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "external-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
}

// createPVCTemplate will create the persistent volume claim template
func createPVCTemplate(volumeName string, stsMeta metav1.ObjectMeta, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	pvcTemplate := storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = volumeName
	pvcTemplate.Labels = stsMeta.GetLabels()
	// We want the same annotations as the StatefulSet here
	pvcTemplate.Annotations = generateStatefulSetsAnots(stsMeta)
	if storageSpec.Spec.AccessModes == nil {
		pvcTemplate.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	} else {
		pvcTemplate.Spec.AccessModes = storageSpec.Spec.AccessModes
	}
	pvcVolumeMode := corev1.PersistentVolumeFilesystem
	if storageSpec.Spec.VolumeMode != nil {
		pvcVolumeMode = *storageSpec.Spec.VolumeMode
	}
	pvcTemplate.Spec.VolumeMode = &pvcVolumeMode
	pvcTemplate.Spec.Resources = storageSpec.Spec.Resources
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	return pvcTemplate
}

// generateContainerDef generates container definition for Redis
func generateContainerDef(name string, containerParams containerParameters, clusterMode, enableMetrics bool, externalConfig *string, mountpath []corev1.VolumeMount, sidecars []v1.Sidecar) []corev1.Container {
	containerDefinition := []corev1.Container{
		{
			Name:            name,
			Image:           containerParams.Image,
			ImagePullPolicy: containerParams.ImagePullPolicy,
			SecurityContext: containerParams.SecurityContext,
			Env: getEnvironmentVariables(
				containerParams.Role,
				false,
				containerParams.EnabledPassword,
				containerParams.SecretName,
				containerParams.SecretKey,
				containerParams.PersistenceEnabled,
				containerParams.RedisExporterEnv,
				containerParams.TLSConfig,
				containerParams.ACLConfig,
			),
			ReadinessProbe: getProbeInfo(containerParams.ReadinessProbe),
			LivenessProbe:  getProbeInfo(containerParams.LivenessProbe),
			VolumeMounts:   getVolumeMount(name, containerParams.PersistenceEnabled, clusterMode, externalConfig, mountpath, containerParams.TLSConfig, containerParams.ACLConfig),
		},
	}

	if containerParams.Resources != nil {
		containerDefinition[0].Resources = *containerParams.Resources
	}
	if enableMetrics {
		containerDefinition = append(containerDefinition, enableRedisMonitoring(containerParams))
	}
	for _, sidecar := range sidecars {
		container := corev1.Container{
			Name:            sidecar.Name,
			Image:           sidecar.Image,
			ImagePullPolicy: sidecar.ImagePullPolicy,
		}
		if sidecar.Command != nil {
			container.Command = sidecar.Command
		}
		if sidecar.Ports != nil {
			container.Ports = append(container.Ports, *sidecar.Ports...)
		}
		if sidecar.Volumes != nil {
			container.VolumeMounts = *sidecar.Volumes
		}
		if sidecar.Resources != nil {
			container.Resources = *sidecar.Resources
		}
		if sidecar.EnvVars != nil {
			container.Env = *sidecar.EnvVars
		}
		containerDefinition = append(containerDefinition, container)
	}

	if containerParams.AdditionalEnvVariable != nil {
		containerDefinition[0].Env = append(containerDefinition[0].Env, *containerParams.AdditionalEnvVariable...)
	}

	return containerDefinition
}

func generateInitContainerDef(name string, initContainerParams initContainerParameters, mountpath []corev1.VolumeMount) []corev1.Container {
	initContainerDefinition := []corev1.Container{
		{
			Name:            "init" + name,
			Image:           initContainerParams.Image,
			ImagePullPolicy: initContainerParams.ImagePullPolicy,
			Command:         initContainerParams.Command,
			Args:            initContainerParams.Arguments,
			VolumeMounts:    getVolumeMount(name, initContainerParams.PersistenceEnabled, false, nil, mountpath, nil, nil),
		},
	}

	if initContainerParams.Resources != nil {
		initContainerDefinition[0].Resources = *initContainerParams.Resources
	}

	if initContainerParams.AdditionalEnvVariable != nil {
		initContainerDefinition[0].Env = append(initContainerDefinition[0].Env, *initContainerParams.AdditionalEnvVariable...)
	}

	return initContainerDefinition
}

func GenerateTLSEnvironmentVariables(tlsConfig *v1.TLSConfig) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	root := "/tls/"

	// get and set Defaults
	caCert := "ca.crt"
	tlsCert := "tls.crt"
	tlsCertKey := "tls.key"

	if tlsConfig.CaKeyFile != "" {
		caCert = tlsConfig.CaKeyFile
	}
	if tlsConfig.CertKeyFile != "" {
		tlsCert = tlsConfig.CertKeyFile
	}
	if tlsConfig.KeyFile != "" {
		tlsCertKey = tlsConfig.KeyFile
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "TLS_MODE",
		Value: "true",
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CA_KEY",
		Value: path.Join(root, caCert),
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CERT",
		Value: path.Join(root, tlsCert),
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CERT_KEY",
		Value: path.Join(root, tlsCertKey),
	})
	return envVars
}

// enableRedisMonitoring will add Redis Exporter as sidecar container
func enableRedisMonitoring(params containerParameters) corev1.Container {
	exporterDefinition := corev1.Container{
		Name:            redisExporterContainer,
		Image:           params.RedisExporterImage,
		ImagePullPolicy: params.RedisExporterImagePullPolicy,
		Env: getEnvironmentVariables(
			params.Role,
			true,
			params.EnabledPassword,
			params.SecretName,
			params.SecretKey,
			params.PersistenceEnabled,
			params.RedisExporterEnv,
			params.TLSConfig,
			params.ACLConfig,
		),
		VolumeMounts: getVolumeMount("", nil, false, nil, params.AdditionalMountPath, params.TLSConfig, params.ACLConfig), // We need/want the tls-certs but we DON'T need the PVC (if one is available)
		Ports: []corev1.ContainerPort{
			{
				//Name:          redisExporterPortName,
				//ContainerPort: redisExporterPort,
				Protocol: corev1.ProtocolTCP,
			},
		},
	}
	if params.RedisExporterResources != nil {
		exporterDefinition.Resources = *params.RedisExporterResources
	}
	return exporterDefinition
}

// getVolumeMount gives information about persistence mount
func getVolumeMount(name string, persistenceEnabled *bool, clusterMode bool, externalConfig *string, mountPath []corev1.VolumeMount, tlsConfig *v1.TLSConfig, aclConfig *v1.ACLConfig) []corev1.VolumeMount {
	var VolumeMounts []corev1.VolumeMount

	if persistenceEnabled != nil && clusterMode {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "node-conf",
			MountPath: "/node-conf",
		})
	}

	if persistenceEnabled != nil && *persistenceEnabled {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: "/data",
		})
	}

	if tlsConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "tls-certs",
			ReadOnly:  true,
			MountPath: "/tls",
		})
	}

	if aclConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "acl-secret",
			MountPath: "/etc/redis/user.acl",
			SubPath:   "user.acl",
		})
	}

	if externalConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "external-config",
			MountPath: "/etc/redis/external.conf.d",
		})
	}

	VolumeMounts = append(VolumeMounts, mountPath...)

	return VolumeMounts
}

// getProbeInfo generate probe for Redis StatefulSet
func getProbeInfo(probe *v1.Probe) *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		FailureThreshold:    probe.FailureThreshold,
		TimeoutSeconds:      probe.TimeoutSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"/usr/bin/healthcheck.sh",
				},
			},
		},
	}
}

// getEnvironmentVariables returns all the required Environment Variables
func getEnvironmentVariables(role string, enabledMetric bool, enabledPassword *bool, secretName *string, secretKey *string, persistenceEnabled *bool, exporterEnvVar *[]corev1.EnvVar, tlsConfig *v1.TLSConfig, aclConfig *v1.ACLConfig) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "SERVER_MODE", Value: role},
		{Name: "SETUP_MODE", Value: role},
	}
	var (
		redisPort    = 6379
		sentinelPort = 26379
	)

	var redisHost string
	if role == "sentinel" {
		redisHost = "redis://localhost:" + strconv.Itoa(sentinelPort)
	} else {
		redisHost = "redis://localhost:" + strconv.Itoa(redisPort)
	}

	if tlsConfig != nil {
		envVars = append(envVars, GenerateTLSEnvironmentVariables(tlsConfig)...)
		if enabledMetric {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE",
				Value: "/tls/tls.key",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE",
				Value: "/tls/tls.crt",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REDIS_EXPORTER_TLS_CA_CERT_FILE",
				Value: "/tls/ca.crt",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
				Value: "true",
			})
		}
	}

	if aclConfig != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ACL_MODE",
			Value: "true",
		})
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_ADDR",
		Value: redisHost,
	})

	if enabledPassword != nil && *enabledPassword {
		envVars = append(envVars, corev1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *secretName,
					},
					Key: *secretKey,
				},
			},
		})
	}
	if persistenceEnabled != nil && *persistenceEnabled {
		envVars = append(envVars, corev1.EnvVar{Name: "PERSISTENCE_ENABLED", Value: "true"})
	}

	if exporterEnvVar != nil {
		envVars = append(envVars, *exporterEnvVar...)
	}
	sort.SliceStable(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}

// createStateFulSet is a method to create stateFulSet in Kubernetes
func createStateFulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := createKubernetesClient().AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful creation failed")
		return err
	}
	logger.Info("Redis stateful successfully created")
	return nil
}

// updateStatefulSet is a method to update stateFulSet in Kubernetes
func updateStatefulSet(namespace string, stateful *appsv1.StatefulSet, recreateStateFulSet bool) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := createKubernetesClient().AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if recreateStateFulSet {
		var sErr *apierror.StatusError
		ok := errors.As(err, &sErr)
		if ok && sErr.ErrStatus.Code == 422 && sErr.ErrStatus.Reason == metav1.StatusReasonInvalid {
			failMsg := make([]string, len(sErr.ErrStatus.Details.Causes))
			for messageCount, cause := range sErr.ErrStatus.Details.Causes {
				failMsg[messageCount] = cause.Message
			}
			logger.Info("recreating StatefulSet because the update operation wasn't possible", "reason", strings.Join(failMsg, ", "))
			propagationPolicy := metav1.DeletePropagationForeground
			if err := createKubernetesClient().AppsV1().StatefulSets(namespace).Delete(context.TODO(), stateful.GetName(), metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil {
				return errors.Wrap(err, "failed to delete StatefulSet to avoid forbidden action")
			}
		}
	}
	if err != nil {
		logger.Error(err, "Redis stateFulSet update failed")
		return err
	}
	logger.Info("Redis stateFulSet successfully updated ")
	return nil
}

// GetStatefulSet is a method to get stateFulSet in Kubernetes
func GetStatefulSet(namespace string, stateful string) (*appsv1.StatefulSet, error) {
	logger := stateFulSetLogger(namespace, stateful)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("StatefulSet", "apps/v1"),
	}
	statefulInfo, err := createKubernetesClient().AppsV1().StatefulSets(namespace).Get(context.TODO(), stateful, getOpts)
	if err != nil {
		logger.Info("Redis stateFulSet get action failed")
		return nil, err
	}
	logger.Info("Redis stateFulSet get action was successful")
	return statefulInfo, nil
}

// stateFulSetLogger will generate logging interface for stateFulSets
func stateFulSetLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.StatefulSet.Namespace", namespace, "Request.StatefulSet.Name", name)
	return reqLogger
}

func getSidecars(sidecars *[]v1.Sidecar) []v1.Sidecar {
	if sidecars == nil {
		return []v1.Sidecar{}
	}
	return *sidecars
}