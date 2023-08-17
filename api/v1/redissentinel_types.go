/*
Copyright 2023 keington.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisSentinelSpec defines the desired state of RedisSentinel
type RedisSentinelSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Size                *int32                     `json:"size"`
	RedisSentinelConfig *RedisSentinelConfig       `json:"redisSentinelConfig,omitempty"`
	RedisConfig         RedisConfig                `json:"redisConfig"`
	NodeSelector        map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext  *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext     *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName   string                     `json:"priorityClassName,omitempty"`
	Affinity            *corev1.Affinity           `json:"affinity,omitempty"`
	Toleration          *[]corev1.Toleration       `json:"toleration,omitempty"`
	TLS                 *TLSConfig                 `json:"TLS,omitempty"`
	PodDisruptionBudget *RedisPodDisruptionBudget  `json:"podDisruptionBudget,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe                 *Probe         `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
	InitContainer                 *InitContainer `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar     `json:"sidecars,omitempty"`
	ServiceAccountName            *string        `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64         `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
}

type RedisSentinelConfig struct {
	AdditionalSentinelConfig *string                          `json:"additionalSentinelConfig,omitempty"`
	Image                    string                           `json:"image"`
	ImagePullPolicy          corev1.PullPolicy                `json:"imagePullPolicy,omitempty"`
	Resources                *corev1.ResourceRequirements     `json:"resources,omitempty"`
	ExistingPasswordSecret   *ExistingPasswordSecret          `json:"redisSecret,omitempty"`
	ImagePullSecrets         *[]corev1.LocalObjectReference   `json:"imagePullSecrets,omitempty"`
	UpdateStrategy           appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Service                  *ServiceConfig                   `json:"service,omitempty"`
	// +kubebuilder:default:=redis-sentinel
	RedisSentinelName string `json:"redisSentinelName"`
	// +kubebuilder:default:=redisSentinelCluster
	MasterGroupName string `json:"masterGroupName,omitempty"`
	// +kubebuilder:default:="26379"
	SentinelPort string `json:"redisPort,omitempty"`
	// +kubebuilder:default:="2"
	Quorum string `json:"quorum,omitempty"`
	// +kubebuilder:default:="1"
	ParallelSyncs string `json:"parallelSyncs,omitempty"`
	// +kubebuilder:default:="180000"
	FailoverTimeout string `json:"failoverTimeout,omitempty"`
	// +kubebuilder:default:="30000"
	DownAfterMilliseconds string `json:"downAfterMilliseconds,omitempty"`
}

func (cr *RedisSentinelSpec) GetSentinelCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

// RedisSentinelStatus defines the observed state of RedisSentinel
type RedisSentinelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// RedisPodDisruptionBudget configure a PodDisruptionBudget on the resource (leader/follower)
type RedisPodDisruptionBudget struct {
	Enabled        bool   `json:"enabled,omitempty"`
	MinAvailable   *int32 `json:"minAvailable,omitempty"`
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RedisSentinel is the Schema for the redis sentinels API
type RedisSentinel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSentinelSpec   `json:"spec,omitempty"`
	Status RedisSentinelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RedisSentinelList contains a list of RedisSentinel
type RedisSentinelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisSentinel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisSentinel{}, &RedisSentinelList{})
}
