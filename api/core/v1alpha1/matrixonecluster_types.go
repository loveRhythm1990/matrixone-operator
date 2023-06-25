// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MatrixOneClusterSpec defines the desired state of MatrixOneCluster
// Note that MatrixOneCluster does not support specify overlay for underlying sets directly due to the size limitation
// of kubernetes apiserver
type MatrixOneClusterSpec struct {
	// TP is the default CN pod set that accepts client connections and execute queries
	// Deprecated: use cnGroups instead
	// +optional
	TP *CNSetSpec `json:"tp,omitempty"`

	// AP is an optional CN pod set that accept MPP sub-plans to accelerate sql queries
	// Deprecated: use cnGroups instead
	// +optional
	AP *CNSetSpec `json:"ap,omitempty"`

	// CNGroups are CN pod sets that have different spec like resources, arch, store labels
	// +listMapKey=name
	// +listType=map
	CNGroups []CNGroup `json:"cnGroups,omitempty"`

	// DN is the default DN pod set of this Cluster
	DN DNSetSpec `json:"dn"`

	// LogService is the default LogService pod set of this cluster
	LogService LogSetSpec `json:"logService"`

	// WebUI is the default web ui pod of this cluster
	// +optional
	WebUI *WebUISpec `json:"webui,omitempty"`

	// Proxy defines an optional MO Proxy of this cluster
	// +optional
	Proxy *ProxySetSpec `json:"proxy,omitempty"`

	// Version is the version of the cluster, which translated
	// to the docker image tag used for each component.
	// default to the recommended version of the operator
	// +required
	Version string `json:"version"`

	// ImageRepository allows user to override the default image
	// repository in order to use a docker registry proxy or private
	// registry.
	// +required
	ImageRepository string `json:"imageRepository,omitempty"`

	// TopologyEvenSpread specifies default topology policy for all components,
	// this will be overridden by component-level config
	// +optional
	TopologyEvenSpread []string `json:"topologySpread,omitempty"`

	// NodeSelector specifies default node selector for all components,
	// this will be overridden by component-level config
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

type CNGroup struct {
	CNSetSpec `json:",inline"`

	// Name is the CNGroup name, an error will be raised if duplicated name is found in a mo cluster
	// +required
	Name string `json:"name"`
}

// MatrixOneClusterStatus defines the observed state of MatrixOneCluster
type MatrixOneClusterStatus struct {
	ConditionalStatus `json:",inline"`

	// Phase is a human-readable description of current cluster condition,
	// programmatic client should rely on ConditionalStatus rather than phase.
	Phase string `json:"phase,omitempty"`

	// CredentialRef is the initial credential of the mo database which can be
	// used to connect to the database.
	CredentialRef *corev1.LocalObjectReference `json:"credentialRef,omitempty"`

	CNGroupStatus CNGroupStatus `json:"cnGroups,omitempty"`
	// TP is the TP set status
	// Deprecated: use
	TP *CNSetStatus `json:"tp,omitempty"`
	// AP is the AP set status
	// Deprecated
	AP *CNSetStatus `json:"ap,omitempty"`
	// DN is the DN set status
	DN *DNSetStatus `json:"dn,omitempty"`
	// Proxy is the Proxy set status
	Proxy *ProxySetStatus `json:"proxy,omitempty"`

	// Webui is the webui service status
	Webui *WebUIStatus `json:"webui,omitempty"`

	// LogService is the LogService status
	LogService *LogSetStatus `json:"logService,omitempty"`

	// Readable is the readable status for human
	Readable *ReadableStatus `json:"readable,omitempty"`
}

type ReadableStatus struct {
	Log string `json:"log,omitempty"`
	DN  string `json:"dn,omitempty"`
	CN  string `json:"cn,omitempty"`
}

type CNGroupStatus struct {
	DesiredGroups int `json:"desiredGroups,omitempty"`
	ReadyGroups   int `json:"readyGroups,omitempty"`
	SyncedGroups  int `json:"syncedGroups,omitempty"`
}

func (s CNGroupStatus) Synced() bool {
	return s.SyncedGroups >= s.DesiredGroups
}

func (s CNGroupStatus) Ready() bool {
	return s.ReadyGroups >= s.DesiredGroups
}

// +kubebuilder:object:root=true

// A MatrixOneCluster is a resource that represents a MatrixOne Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mo
// +kubebuilder:printcolumn:name="Log",type="integer",JSONPath=".spec.logService.replicas"
// +kubebuilder:printcolumn:name="DN",type="integer",JSONPath=".spec.dn.replicas"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MatrixOneCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of MatrixOneCluster
	Spec MatrixOneClusterSpec `json:"spec"`

	// Status is the current state of MatrixOneCluster
	Status MatrixOneClusterStatus `json:"status,omitempty"`
}

func (mo *MatrixOneCluster) SetCondition(condition metav1.Condition) {
	mo.Status.SetCondition(condition)
}

func (mo *MatrixOneCluster) GetConditions() []metav1.Condition {
	return mo.Status.GetConditions()
}

//+kubebuilder:object:root=true

// MatrixOneClusterList contains a list of MatrixOneCluster
type MatrixOneClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MatrixOneCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MatrixOneCluster{}, &MatrixOneClusterList{})
}
