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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testModifiedURL = "https://modified.example.com"

func TestExternalSourceDeepCopy(t *testing.T) {
	original := &ExternalSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-source",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: ExternalSourceSpec{
			Interval: "5m",
			Suspend:  false,
			Generator: GeneratorSpec{
				Type: "http",
				HTTP: &HTTPGeneratorSpec{
					URL:    "https://api.example.com/data",
					Method: "GET",
				},
			},
			DestinationPath: "config.json",
		},
	}

	// Test DeepCopy
	copied := original.DeepCopy()

	// Verify it's a different object
	assert.NotSame(t, original, copied)

	// Verify content is the same
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Namespace, copied.Namespace)
	assert.Equal(t, original.Spec.Interval, copied.Spec.Interval)
	assert.Equal(t, original.Spec.Generator.Type, copied.Spec.Generator.Type)

	// Verify deep copy (modifying copy doesn't affect original)
	copied.Name = "modified-name"
	copied.Spec.Generator.HTTP.URL = testModifiedURL

	assert.NotEqual(t, original.Name, copied.Name)
	assert.NotEqual(t, original.Spec.Generator.HTTP.URL, copied.Spec.Generator.HTTP.URL)

	// Test DeepCopyObject
	obj := original.DeepCopyObject()
	copiedObj, ok := obj.(*ExternalSource)
	assert.True(t, ok)
	assert.Equal(t, original.Name, copiedObj.Name)
}

func TestExternalArtifactDeepCopy(t *testing.T) {
	original := &ExternalArtifact{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-artifact",
			Namespace: "test-namespace",
		},
		Spec: ExternalArtifactSpec{
			URL:      "https://storage.example.com/artifact.tar.gz",
			Revision: "abc123def456",
			Metadata: map[string]string{
				"contentType": "application/gzip",
			},
		},
	}

	// Test DeepCopy
	copied := original.DeepCopy()

	// Verify it's a different object
	assert.NotSame(t, original, copied)
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Spec.URL, copied.Spec.URL)

	// Verify deep copy
	copied.Name = "modified-artifact"
	copied.Spec.Metadata["new"] = "value"

	assert.NotEqual(t, original.Name, copied.Name)
	assert.NotContains(t, original.Spec.Metadata, "new")

	// Test DeepCopyObject
	obj := original.DeepCopyObject()
	copiedObj, ok := obj.(*ExternalArtifact)
	assert.True(t, ok)
	assert.Equal(t, original.Spec.URL, copiedObj.Spec.URL)
}

func TestExternalSourceSpecDeepCopy(t *testing.T) {
	original := ExternalSourceSpec{
		Interval: "10m",
		Suspend:  true,
		Generator: GeneratorSpec{
			Type: "http",
			HTTP: &HTTPGeneratorSpec{
				URL:    "https://api.example.com/data",
				Method: "POST",
			},
		},
		Hooks: &HooksSpec{
			PostRequest: []HookSpec{
				{
					Name:        "transform-jq",
					Command:     "jq",
					Args:        []string{".items"},
					Timeout:     "30s",
					RetryPolicy: "fail",
				},
			},
		},
		MaxRetries:      3,
		DestinationPath: "items.json",
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, &original, copied)
	assert.Equal(t, original.Interval, copied.Interval)
	assert.Equal(t, original.Generator.Type, copied.Generator.Type)

	// Modify copy and verify original is unchanged
	copied.Generator.HTTP.URL = testModifiedURL
	copied.Hooks.PostRequest[0].Command = "modified-command"

	assert.NotEqual(t, original.Generator.HTTP.URL, copied.Generator.HTTP.URL)
	assert.NotEqual(t, original.Hooks.PostRequest[0].Command, copied.Hooks.PostRequest[0].Command)
}

func TestGeneratorSpecDeepCopy(t *testing.T) {
	original := GeneratorSpec{
		Type: "http",
		HTTP: &HTTPGeneratorSpec{
			URL:    "https://api.example.com/data",
			Method: "GET",
		},
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, &original, copied)
	assert.Equal(t, original.Type, copied.Type)
	assert.NotSame(t, original.HTTP, copied.HTTP)
	assert.Equal(t, original.HTTP.URL, copied.HTTP.URL)

	// Modify copy and verify original is unchanged
	copied.HTTP.URL = testModifiedURL

	assert.NotEqual(t, original.HTTP.URL, copied.HTTP.URL)
}

func TestHTTPGeneratorSpecDeepCopy(t *testing.T) {
	original := HTTPGeneratorSpec{
		URL:                "https://api.example.com/data",
		Method:             "POST",
		InsecureSkipVerify: true,
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, &original, copied)
	assert.Equal(t, original.URL, copied.URL)
	assert.Equal(t, original.Method, copied.Method)

	// Modify copy and verify original is unchanged
	copied.URL = testModifiedURL

	assert.NotEqual(t, original.URL, copied.URL)
}

func TestHooksSpecDeepCopy(t *testing.T) {
	original := HooksSpec{
		PreRequest: []HookSpec{
			{
				Name:        "prepare",
				Command:     "bash",
				Args:        []string{"-c", "echo hello"},
				Timeout:     "30s",
				RetryPolicy: "fail",
			},
		},
		PostRequest: []HookSpec{
			{
				Name:        "transform",
				Command:     "jq",
				Args:        []string{".data"},
				Timeout:     "30s",
				RetryPolicy: "retry",
			},
		},
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, &original, copied)
	assert.Equal(t, len(original.PreRequest), len(copied.PreRequest))
	assert.Equal(t, original.PreRequest[0].Command, copied.PreRequest[0].Command)
	assert.Equal(t, len(original.PostRequest), len(copied.PostRequest))
	assert.Equal(t, original.PostRequest[0].Command, copied.PostRequest[0].Command)

	// Modify copy and verify original is unchanged
	copied.PostRequest[0].Command = "modified-command"

	assert.NotEqual(t, original.PostRequest[0].Command, copied.PostRequest[0].Command)
}

func TestSecretReferenceDeepCopy(t *testing.T) {
	original := SecretReference{
		Name: "test-secret",
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, &original, copied)
	assert.Equal(t, original.Name, copied.Name)

	// Modify copy and verify original is unchanged
	copied.Name = "modified-secret"

	assert.NotEqual(t, original.Name, copied.Name)
}

func TestExternalSourceListDeepCopy(t *testing.T) {
	original := &ExternalSourceList{
		Items: []ExternalSource{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "source1"},
				Spec:       ExternalSourceSpec{Interval: "5m"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "source2"},
				Spec:       ExternalSourceSpec{Interval: "10m"},
			},
		},
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, original, copied)
	assert.Len(t, copied.Items, 2)
	assert.Equal(t, original.Items[0].Name, copied.Items[0].Name)

	// Modify copy and verify original is unchanged
	copied.Items[0].Name = "modified-source1"

	assert.NotEqual(t, original.Items[0].Name, copied.Items[0].Name)

	// Test DeepCopyObject
	obj := original.DeepCopyObject()
	copiedObj, ok := obj.(*ExternalSourceList)
	assert.True(t, ok)
	assert.Len(t, copiedObj.Items, 2)
}

func TestExternalArtifactListDeepCopy(t *testing.T) {
	original := &ExternalArtifactList{
		Items: []ExternalArtifact{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "artifact1"},
				Spec:       ExternalArtifactSpec{URL: "https://example.com/artifact1.tar.gz"},
			},
		},
	}

	copied := original.DeepCopy()

	// Verify deep copy
	assert.NotSame(t, original, copied)
	assert.Len(t, copied.Items, 1)
	assert.Equal(t, original.Items[0].Spec.URL, copied.Items[0].Spec.URL)

	// Test DeepCopyObject
	obj := original.DeepCopyObject()
	copiedObj, ok := obj.(*ExternalArtifactList)
	assert.True(t, ok)
	assert.Len(t, copiedObj.Items, 1)
}
