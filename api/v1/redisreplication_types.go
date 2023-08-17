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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisReplicationSpec struct {
	Size               *int32                     `json:"clusterSize"`
	RedisConfig        *RedisConfig               `json:"redisConfig,omitempty"`
	Storage            *Storage                   `json:"storage,omitempty"`
	NodeSelector       map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext    *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName  string                     `json:"priorityClassName,omitempty"`
	Affinity           *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations        *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS                *TLSConfig                 `json:"TLS,omitempty"`
	ACL                *ACLConfig                 `json:"acl,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe                 *Probe         `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
	InitContainer                 *InitContainer `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar     `json:"sidecars,omitempty"`
	ServiceAccountName            *string        `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64         `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
}

func (cr *RedisReplicationSpec) GetReplicationCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

// RedisReplicationStatus RedisStatus defines the observed state of Redis
type RedisReplicationStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RedisReplication Redis is the Schema for the redis API
type RedisReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisReplicationSpec   `json:"spec"`
	Status RedisReplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisReplicationList RedisList contains a list of RedisReplication
type RedisReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisReplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisReplication{}, &RedisReplicationList{})
}
