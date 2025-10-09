/*
Copyright 2025.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExternalArtifactSpec defines the desired state of ExternalArtifact
type ExternalArtifactSpec struct {
	// URL is the location where the artifact can be accessed
	// +required
	URL string `json:"url"`

	// Revision is the content-based revision of the artifact
	// +required
	Revision string `json:"revision"`

	// Metadata contains additional artifact metadata
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ExternalArtifactStatus defines the observed state of ExternalArtifact
type ExternalArtifactStatus struct {
	// Conditions represent the current state of the ExternalArtifact resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last observed generation of the ExternalArtifact
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".spec.revision"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ExternalArtifact is the Schema for the externalartifacts API
type ExternalArtifact struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ExternalArtifact
	// +required
	Spec ExternalArtifactSpec `json:"spec"`

	// status defines the observed state of ExternalArtifact
	// +optional
	Status ExternalArtifactStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ExternalArtifactList contains a list of ExternalArtifact
type ExternalArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalArtifact `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalArtifact{}, &ExternalArtifactList{})
}
