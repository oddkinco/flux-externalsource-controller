/*
Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
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
