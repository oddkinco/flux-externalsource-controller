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

// ExternalSourceSpec defines the desired state of ExternalSource
type ExternalSourceSpec struct {
	// Interval specifies the reconciliation frequency
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ms|s|m|h))+$`
	// +kubebuilder:validation:MinLength=2
	// +required
	Interval string `json:"interval"`

	// Suspend tells the controller to suspend reconciliation for this ExternalSource
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// DestinationPath specifies the relative path within the artifact where the data should be placed
	// +optional
	DestinationPath string `json:"destinationPath,omitempty"`

	// Transform specifies optional data transformation
	// +optional
	Transform *TransformSpec `json:"transform,omitempty"`

	// Generator specifies the source generator configuration
	// +required
	Generator GeneratorSpec `json:"generator"`
}

// TransformSpec defines data transformation configuration
type TransformSpec struct {
	// Type specifies the transformation engine type
	// +kubebuilder:validation:Enum=cel
	// +required
	Type string `json:"type"`

	// Expression contains the transformation expression
	// +required
	Expression string `json:"expression"`
}

// GeneratorSpec defines the source generator configuration
type GeneratorSpec struct {
	// Type specifies the generator type
	// +kubebuilder:validation:Enum=http
	// +required
	Type string `json:"type"`

	// HTTP specifies HTTP generator configuration
	// +optional
	HTTP *HTTPGeneratorSpec `json:"http,omitempty"`
}

// HTTPGeneratorSpec defines HTTP source generator configuration
type HTTPGeneratorSpec struct {
	// URL is the HTTP endpoint to fetch data from
	// +kubebuilder:validation:Format=uri
	// +required
	URL string `json:"url"`

	// Method specifies the HTTP method to use
	// +kubebuilder:default=GET
	// +optional
	Method string `json:"method,omitempty"`

	// HeadersSecretRef references a secret containing HTTP headers
	// +optional
	HeadersSecretRef *SecretReference `json:"headersSecretRef,omitempty"`

	// CABundleSecretRef references a secret containing a CA bundle for TLS verification
	// +optional
	CABundleSecretRef *SecretKeyReference `json:"caBundleSecretRef,omitempty"`

	// InsecureSkipVerify skips TLS certificate verification (not recommended for production)
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// SecretReference contains the name of a secret
type SecretReference struct {
	// Name of the secret
	// +required
	Name string `json:"name"`
}

// SecretKeyReference contains the name of a secret and a key within that secret
type SecretKeyReference struct {
	// Name of the secret
	// +required
	Name string `json:"name"`

	// Key within the secret
	// +required
	Key string `json:"key"`
}

// ExternalSourceStatus defines the observed state of ExternalSource
type ExternalSourceStatus struct {
	// Conditions represent the current state of the ExternalSource resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Artifact contains information about the current artifact
	// +optional
	Artifact *ArtifactMetadata `json:"artifact,omitempty"`

	// LastHandledETag contains the ETag from the last successful fetch (for HTTP sources)
	// +optional
	LastHandledETag string `json:"lastHandledETag,omitempty"`

	// ObservedGeneration is the last observed generation of the ExternalSource
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ArtifactMetadata contains metadata about an artifact
type ArtifactMetadata struct {
	// URL is the location where the artifact can be accessed
	// +required
	URL string `json:"url"`

	// Revision is the content-based revision of the artifact
	// +required
	Revision string `json:"revision"`

	// LastUpdateTime is when the artifact was last updated
	// +required
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// Metadata contains additional artifact metadata
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ExternalSource is the Schema for the externalsources API
type ExternalSource struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ExternalSource
	// +required
	Spec ExternalSourceSpec `json:"spec"`

	// status defines the observed state of ExternalSource
	// +optional
	Status ExternalSourceStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ExternalSourceList contains a list of ExternalSource
type ExternalSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalSource{}, &ExternalSourceList{})
}
