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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	matev1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	v1 "redis-sentinel/api/v1"
)

// RedisSentinelSTS is a interface to call Redis StatefulSet function
type RedisSentinelSTS struct {
	RedisStateFulType             string
	ExternalConfig                *string
	Affinity                      *corev1.Affinity `json:"affinity,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty"`
	ReadinessProbe                *v1.Probe
	LivenessProbe                 *v1.Probe
}

// RedisSentinelService is a interface to call Redis Service function
type RedisSentinelService struct {
	RedisService string
}

// CreateRedisSentinel Redis Sentinel Create the Redis Sentinel Setup
func CreateRedisSentinel(cr *v1.RedisSentinel) error {

	prop := RedisSentinelSTS{
		RedisStateFulType:             cr.Name,
		Affinity:                      cr.Spec.Affinity,
		ReadinessProbe:                cr.Spec.ReadinessProbe,
		LivenessProbe:                 cr.Spec.LivenessProbe,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
	}

	if cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig
	}

	return prop.CreateRedisSentinelSetup(cr)
}

// CreateRedisSentinelService Create RedisSentinel Service
func CreateRedisSentinelService(cr *v1.RedisSentinel) error {

	prop := RedisSentinelService{
		RedisService: "sentinel",
	}
	return prop.CreateRedisSentinelService(cr)
}

// CreateRedisSentinelSetup Create Redis Sentinel Cluster Setup
func (service RedisSentinelSTS) CreateRedisSentinelSetup(cr *v1.RedisSentinel) error {

	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := stateFulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, "cluster", service.RedisStateFulType, cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		cr.Namespace,
		objectMetaInfo,
		generateRedisSentinelParams(cr, service.getSentinelCount(cr), service.ExternalConfig, service.Affinity),
		redisSentinelAsOwner(cr),
		generateRedisSentinelInitContainerParams(cr),
		generateRedisSentinelContainerParams(cr, service.ReadinessProbe, service.LivenessProbe),
		cr.Spec.Sidecars,
	)

	if err != nil {
		logger.Error(err, "Cannot create Sentinel statefulset for Redis")
		return err
	}
	return nil
}

// Create Redis Sentinel Params for the stateFulSet
func generateRedisSentinelParams(cr *v1.RedisSentinel, replicas int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {

	res := statefulSetParameters{
		Metadata:                      cr.ObjectMeta,
		Replicas:                      &replicas,
		ClusterMode:                   false,
		NodeSelector:                  cr.Spec.NodeSelector,
		PodSecurityContext:            cr.Spec.PodSecurityContext,
		PriorityClassName:             cr.Spec.PriorityClassName,
		Affinity:                      affinity,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
		Tolerations:                   cr.Spec.Toleration,
		ServiceAccountName:            cr.Spec.ServiceAccountName,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
	}

	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if externalConfig != nil {
		res.ExternalConfig = externalConfig
	}
	return res
}

// generateRedisSentinelInitContainerParams generates Redis sentinel initContainer information
func generateRedisSentinelInitContainerParams(cr *v1.RedisSentinel) initContainerParameters {

	initContainerProp := initContainerParameters{}

	if cr.Spec.InitContainer != nil {
		initContainer := cr.Spec.InitContainer

		initContainerProp = initContainerParameters{
			Enabled:               initContainer.Enabled,
			Role:                  "sentinel",
			Image:                 initContainer.Image,
			ImagePullPolicy:       initContainer.ImagePullPolicy,
			Resources:             initContainer.Resources,
			AdditionalEnvVariable: initContainer.EnvVars,
			Command:               initContainer.Command,
			Arguments:             initContainer.Args,
		}

	}

	return initContainerProp
}

// Create Redis Sentinel Statefulset Container Params
func generateRedisSentinelContainerParams(cr *v1.RedisSentinel, readinessProbeDef *v1.Probe, livenessProbeDef *v1.Probe) containerParameters {

	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                  "sentinel",
		Image:                 cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:       cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:             cr.Spec.KubernetesConfig.Resources,
		SecurityContext:       cr.Spec.SecurityContext,
		AdditionalEnvVariable: getSentinelEnvVariable(cr),
	}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key
	} else {
		containerProp.EnabledPassword = &falseProperty
	}
	if readinessProbeDef != nil {
		containerProp.ReadinessProbe = readinessProbeDef
	}
	if livenessProbeDef != nil {
		containerProp.LivenessProbe = livenessProbeDef
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}

	return containerProp

}

// Get the Count of the Sentinel
func (service RedisSentinelSTS) getSentinelCount(cr *v1.RedisSentinel) int32 {
	return cr.Spec.GetSentinelCounts(service.RedisStateFulType)
}

// CreateRedisSentinelService Create the Service for redis sentinel
func (service RedisSentinelService) CreateRedisSentinelService(cr *v1.RedisSentinel) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisService
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "cluster", service.RedisService, cr.ObjectMeta.Labels)
	annotations := generateServiceAnots(cr.ObjectMeta, nil)

	additionalServiceAnnotations := map[string]string{}
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceAnnotations = cr.Spec.KubernetesConfig.Service.ServiceAnnotations
	}

	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(serviceName+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, additionalServiceAnnotations))

	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisSentinelAsOwner(cr), false, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisService)
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisSentinelAsOwner(cr), false, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisService)
		return err
	}

	additionalServiceType := "ClusterIP"
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceType = cr.Spec.KubernetesConfig.Service.ServiceType
	}
	err = CreateOrUpdateService(cr.Namespace, additionalObjectMetaInfo, redisSentinelAsOwner(cr), false, additionalServiceType)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisService)
		return err
	}
	return nil

}

func getSentinelEnvVariable(cr *v1.RedisSentinel) *[]corev1.EnvVar {

	envVar := &[]corev1.EnvVar{
		{
			Name:  "MASTER_GROUP_NAME",
			Value: cr.Spec.RedisSentinelConfig.MasterGroupName,
		},
		{
			Name:  "IP",
			Value: getRedisReplicationMasterIP(cr),
		},
		{
			Name:  "PORT",
			Value: cr.Spec.RedisSentinelConfig.SentinelPort,
		},
		{
			Name:  "QUORUM",
			Value: cr.Spec.RedisSentinelConfig.Quorum,
		},
		{
			Name:  "DOWN_AFTER_MILLISECONDS",
			Value: cr.Spec.RedisSentinelConfig.DownAfterMilliseconds,
		},
		{
			Name:  "PARALLEL_SYNCS",
			Value: cr.Spec.RedisSentinelConfig.ParallelSyncs,
		},
		{
			Name:  "FAILOVER_TIMEOUT",
			Value: cr.Spec.RedisSentinelConfig.FailoverTimeout,
		},
	}

	return envVar

}

func getRedisReplicationMasterIP(cr *v1.RedisSentinel) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	replicationName := cr.Spec.RedisSentinelConfig.RedisSentinelName
	replicationNamespace := cr.Namespace

	var replicationInstance v1.RedisSentinel
	var realMasterPod string

	// Get Request on Dynamic Client
	customObject, err := createK8sDynamicClient().Resource(schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta1",
		Resource: "redisreplications",
	}).Namespace(replicationNamespace).Get(context.TODO(), replicationName, matev1.GetOptions{})

	if err != nil {
		logger.Error(err, "Failed to Execute Get Request", "replication name", replicationName, "namespace", replicationNamespace)
		return ""
	} else {
		logger.Info("Successfully Execute the Get Request", "replication name", replicationName, "namespace", replicationNamespace)
	}

	// Marshal CustomObject to JSON
	replicationJSON, err := customObject.MarshalJSON()
	if err != nil {
		logger.Error(err, "Failed To Load JSON")
		return ""
	}

	// Unmarshal The JSON on Object
	if err := json.Unmarshal(replicationJSON, &replicationInstance); err != nil {
		logger.Error(err, "Failed To Unmarshal JSON over the Object")
		return ""
	}

	masterPods := GetRedisNodesByRole(&replicationInstance, "master")

	if len(masterPods) == 0 {
		realMasterPod = ""
		err := errors.New("no master pods found")
		logger.Error(err, "")
	} else if len(masterPods) == 1 {
		realMasterPod = masterPods[0]
	} else {
		realMasterPod = checkAttachedSlave(&replicationInstance, masterPods)
	}

	realMasterInfo := RedisDetails{
		PodName:   realMasterPod,
		Namespace: replicationNamespace,
	}

	realMasterPodIP := getRedisServerIP(realMasterInfo)
	return realMasterPodIP
}

// generateRedisManagerLogger will generate logging interface for Redis operations
func generateRedisManagerLogger(namespace, name string) logr.Logger {
	reqLogger := log.WithValues("Request.RedisManager.Namespace", namespace, "Request.RedisManager.Name", name)
	return reqLogger
}
