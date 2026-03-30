/*
Copyright 2026.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagedNamespaceConfigurationSpec defines the desired state of ManagedNamespaceConfiguration
type ManagedNamespaceConfigurationSpec struct {
	// +optional
	Resources []Resources `json:"resources,omitempty"`
	// +optional
	Callbacks []Callbacks `json:"callbacks,omitempty"`
	// +optional
	Suspended bool `json:"suspended,omitempty"`
}

// Resources defines the list of managed resource.
type Resources struct {
	// +required
	Resource Resource `json:"resource"`
	// +required
	Content string `json:"content"`
}

// Resource defines the managed resource.
type Resource struct {
	ApiVersion string `json:"apiVersion,omitempty"`
	// +required
	Kind string `json:"kind"`
	// +required
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +optional
	ClusterResource bool `json:"clusterresource,omitempty"`
}

// Callbacks defines the list of webhook/callback configuration.
type Callbacks struct {
	// +required
	URI string `json:"uri"`
	// +optional
	Method string `json:"method,omitempty"`
	// +optional
	SuccessCodes []int `json:"successcodes,omitempty"`
	// +optional
	CACert string `json:"cacert,omitempty"`
	// +optional
	Headers []HTTPHeader `json:"headers,omitempty"`
}

// Resources defines an HTTP Header for callback.
type HTTPHeader struct {
	// +required
	Name string `json:"name"`
	// +required
	Value string `json:"value"`
}

// ManagedNamespaceConfigurationStatus defines the observed state of ManagedNamespaceConfiguration.
type ManagedNamespaceConfigurationStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ManagedNamespaceConfiguration resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LastSyncTime metav1.Time `json:"lastSyncTime"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ManagedNamespaceConfiguration is the Schema for the managednamespaceconfigurations API
type ManagedNamespaceConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ManagedNamespaceConfiguration
	// +optional
	Spec ManagedNamespaceConfigurationSpec `json:"spec"`

	// status defines the observed state of ManagedNamespaceConfiguration
	// +optional
	Status ManagedNamespaceConfigurationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ManagedNamespaceConfigurationList contains a list of ManagedNamespaceConfiguration
type ManagedNamespaceConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ManagedNamespaceConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedNamespaceConfiguration{}, &ManagedNamespaceConfigurationList{})
}
