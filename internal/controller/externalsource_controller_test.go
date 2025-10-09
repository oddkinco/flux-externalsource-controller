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

package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sourcev1alpha1 "github.com/example/externalsource-controller/api/v1alpha1"
	"github.com/example/externalsource-controller/internal/artifact"
	"github.com/example/externalsource-controller/internal/generator"
	"github.com/example/externalsource-controller/internal/transformer"
)

var _ = Describe("ExternalSource Controller", func() {
	Context("CRD Validation", func() {
		ctx := context.Background()

		Describe("Valid ExternalSource configurations", func() {
			It("should accept a valid HTTP generator configuration", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-http-source",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL:    "https://api.example.com/config",
								Method: "GET",
							},
						},
					},
				}

				Expect(k8sClient.Create(ctx, externalSource)).To(Succeed())

				// Cleanup
				Expect(k8sClient.Delete(ctx, externalSource)).To(Succeed())
			})

			It("should accept HTTP generator with authentication", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "http-with-auth",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "10m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL:    "https://secure-api.example.com/config",
								Method: "POST",
								HeadersSecretRef: &sourcev1alpha1.SecretReference{
									Name: "api-headers",
								},
								CABundleSecretRef: &sourcev1alpha1.SecretKeyReference{
									Name: "ca-bundle",
									Key:  "ca.crt",
								},
							},
						},
					},
				}

				Expect(k8sClient.Create(ctx, externalSource)).To(Succeed())

				// Cleanup
				Expect(k8sClient.Delete(ctx, externalSource)).To(Succeed())
			})

			It("should accept configuration with transformation", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "with-transform",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "1m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL: "https://api.example.com/data",
							},
						},
						Transform: &sourcev1alpha1.TransformSpec{
							Type:       "cel",
							Expression: "data.config",
						},
						DestinationPath: "config/app.json",
					},
				}

				Expect(k8sClient.Create(ctx, externalSource)).To(Succeed())

				// Cleanup
				Expect(k8sClient.Delete(ctx, externalSource)).To(Succeed())
			})
		})

		Describe("Invalid ExternalSource configurations", func() {
			It("should reject missing required interval field", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "missing-interval",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL: "https://api.example.com/config",
							},
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("interval"))
			})

			It("should reject invalid interval format", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-interval",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "x", // Invalid format
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL: "https://api.example.com/config",
							},
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
			})

			It("should reject missing generator field", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "missing-generator",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("generator"))
			})

			It("should reject invalid generator type", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-generator-type",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "invalid", // Not in enum
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
			})

			It("should reject HTTP generator without URL", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "http-no-url",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								Method: "GET", // Missing required URL
							},
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("url"))
			})

			It("should reject invalid URL format", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-url",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL: "not-a-valid-url", // Invalid URI format
							},
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
			})

			It("should reject invalid transformation type", func() {
				externalSource := &sourcev1alpha1.ExternalSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-transform-type",
						Namespace: "default",
					},
					Spec: sourcev1alpha1.ExternalSourceSpec{
						Interval: "5m",
						Generator: sourcev1alpha1.GeneratorSpec{
							Type: "http",
							HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
								URL: "https://api.example.com/config",
							},
						},
						Transform: &sourcev1alpha1.TransformSpec{
							Type:       "invalid", // Not in enum
							Expression: "data.config",
						},
					},
				}

				err := k8sClient.Create(ctx, externalSource)
				Expect(err).To(HaveOccurred())
			})


		})
	})

	Context("When reconciling a resource", func() {
		ctx := context.Background()

		It("should successfully reconcile the resource", func() {
			resourceName := "test-reconcile-success"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("creating a valid ExternalSource resource")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("reconciling the created resource")
			controllerReconciler := &ExternalSourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// First reconcile adds finalizer and requeues
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile performs actual reconciliation
			result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Minute)) // 5 minutes

			By("verifying the status was updated")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())
			Expect(updatedResource.Status.ObservedGeneration).To(Equal(updatedResource.Generation))
			Expect(updatedResource.Status.Conditions).NotTo(BeEmpty())

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle suspended resources", func() {
			resourceName := "test-reconcile-suspended"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("creating a suspended ExternalSource resource")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Suspend:  true,
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("reconciling the suspended resource")
			controllerReconciler := &ExternalSourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// First reconcile adds finalizer and requeues
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile handles suspension
			result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			By("verifying the suspended condition")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			readyCondition := findCondition(updatedResource.Status.Conditions, ReadyCondition)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal(SuspendedReason))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})

// Mock implementations for testing

// MockSourceGenerator implements generator.SourceGenerator for testing
type MockSourceGenerator struct {
	GenerateFunc               func(ctx context.Context, config generator.GeneratorConfig) (*generator.SourceData, error)
	SupportsConditionalFetchFunc func() bool
	GetLastModifiedFunc        func(ctx context.Context, config generator.GeneratorConfig) (string, error)
}

func (m *MockSourceGenerator) Generate(ctx context.Context, config generator.GeneratorConfig) (*generator.SourceData, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, config)
	}
	return &generator.SourceData{
		Data:         []byte(`{"test": "data"}`),
		LastModified: "test-etag",
		Metadata:     map[string]string{"content-type": "application/json"},
	}, nil
}

func (m *MockSourceGenerator) SupportsConditionalFetch() bool {
	if m.SupportsConditionalFetchFunc != nil {
		return m.SupportsConditionalFetchFunc()
	}
	return true
}

func (m *MockSourceGenerator) GetLastModified(ctx context.Context, config generator.GeneratorConfig) (string, error) {
	if m.GetLastModifiedFunc != nil {
		return m.GetLastModifiedFunc(ctx, config)
	}
	return "test-etag", nil
}

// MockGeneratorFactory implements generator.SourceGeneratorFactory for testing
type MockGeneratorFactory struct {
	CreateGeneratorFunc    func(generatorType string) (generator.SourceGenerator, error)
	RegisterGeneratorFunc  func(generatorType string, factory func() generator.SourceGenerator) error
	SupportedTypesFunc     func() []string
	generators             map[string]func() generator.SourceGenerator
}

func NewMockGeneratorFactory() *MockGeneratorFactory {
	return &MockGeneratorFactory{
		generators: make(map[string]func() generator.SourceGenerator),
	}
}

func (m *MockGeneratorFactory) CreateGenerator(generatorType string) (generator.SourceGenerator, error) {
	if m.CreateGeneratorFunc != nil {
		return m.CreateGeneratorFunc(generatorType)
	}
	if factory, exists := m.generators[generatorType]; exists {
		return factory(), nil
	}
	return nil, fmt.Errorf("unsupported generator type: %s", generatorType)
}

func (m *MockGeneratorFactory) RegisterGenerator(generatorType string, factory func() generator.SourceGenerator) error {
	if m.RegisterGeneratorFunc != nil {
		return m.RegisterGeneratorFunc(generatorType, factory)
	}
	m.generators[generatorType] = factory
	return nil
}

func (m *MockGeneratorFactory) SupportedTypes() []string {
	if m.SupportedTypesFunc != nil {
		return m.SupportedTypesFunc()
	}
	supportedTypes := make([]string, 0, len(m.generators))
	for t := range m.generators {
		supportedTypes = append(supportedTypes, t)
	}
	return supportedTypes
}

// MockTransformer implements transformer.Transformer for testing
type MockTransformer struct {
	TransformFunc func(ctx context.Context, input []byte, expression string) ([]byte, error)
}

// Ensure MockTransformer implements transformer.Transformer
var _ transformer.Transformer = (*MockTransformer)(nil)

func (m *MockTransformer) Transform(ctx context.Context, input []byte, expression string) ([]byte, error) {
	if m.TransformFunc != nil {
		return m.TransformFunc(ctx, input, expression)
	}
	// Default transformation just returns the input
	return input, nil
}

// MockArtifactManager implements artifact.ArtifactManager for testing
type MockArtifactManager struct {
	PackageFunc func(ctx context.Context, data []byte, path string) (*artifact.Artifact, error)
	StoreFunc   func(ctx context.Context, artifact *artifact.Artifact, source string) (string, error)
	CleanupFunc func(ctx context.Context, source string, keepRevision string) error
}

func (m *MockArtifactManager) Package(ctx context.Context, data []byte, path string) (*artifact.Artifact, error) {
	if m.PackageFunc != nil {
		return m.PackageFunc(ctx, data, path)
	}
	return &artifact.Artifact{
		Data:     data,
		Path:     path,
		Revision: "test-revision-123",
		Metadata: map[string]string{"size": fmt.Sprintf("%d", len(data))},
	}, nil
}

func (m *MockArtifactManager) Store(ctx context.Context, art *artifact.Artifact, source string) (string, error) {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, art, source)
	}
	return fmt.Sprintf("https://storage.example.com/%s/%s", source, art.Revision), nil
}

func (m *MockArtifactManager) Cleanup(ctx context.Context, source string, keepRevision string) error {
	if m.CleanupFunc != nil {
		return m.CleanupFunc(ctx, source, keepRevision)
	}
	return nil
}

// Integration tests for reconciliation logic
var _ = Describe("ExternalSource Controller Integration", func() {
	Context("Complete reconciliation flow", func() {
		var (
			ctx                 context.Context
			mockFactory         *MockGeneratorFactory
			mockTransformer     *MockTransformer
			mockArtifactManager *MockArtifactManager
			reconciler          *ExternalSourceReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			mockFactory = NewMockGeneratorFactory()
			mockTransformer = &MockTransformer{}
			mockArtifactManager = &MockArtifactManager{}

			reconciler = &ExternalSourceReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				GeneratorFactory: mockFactory,
				Transformer:      mockTransformer,
				ArtifactManager:  mockArtifactManager,
			}

			// Register mock HTTP generator
			Expect(mockFactory.RegisterGenerator("http", func() generator.SourceGenerator {
				return &MockSourceGenerator{}
			})).To(Succeed())
		})

		It("should successfully complete full reconciliation flow", func() {
			resourceName := "test-full-reconciliation"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("creating an ExternalSource resource")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
					Transform: &sourcev1alpha1.TransformSpec{
						Type:       "cel",
						Expression: "data.config",
					},
					DestinationPath: "config/app.json",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("performing reconciliation")
			// First reconcile adds finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile performs actual work
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Minute))

			By("verifying the status was updated correctly")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			// Check conditions
			readyCondition := findCondition(updatedResource.Status.Conditions, ReadyCondition)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal(SucceededReason))

			// Check artifact metadata
			Expect(updatedResource.Status.Artifact).NotTo(BeNil())
			Expect(updatedResource.Status.Artifact.URL).To(ContainSubstring("storage.example.com"))
			Expect(updatedResource.Status.Artifact.Revision).To(Equal("test-revision-123"))

			// Check ETag was stored
			Expect(updatedResource.Status.LastHandledETag).To(Equal("test-etag"))

			By("verifying ExternalArtifact child resource was created")
			var externalArtifact sourcev1alpha1.ExternalArtifact
			artifactKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			Expect(k8sClient.Get(ctx, artifactKey, &externalArtifact)).To(Succeed())
			Expect(externalArtifact.Spec.URL).To(Equal(updatedResource.Status.Artifact.URL))
			Expect(externalArtifact.Spec.Revision).To(Equal(updatedResource.Status.Artifact.Revision))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle conditional fetching with unchanged ETag", func() {
			resourceName := "test-conditional-fetch"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("creating an ExternalSource resource with existing ETag")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
				},
				Status: sourcev1alpha1.ExternalSourceStatus{
					LastHandledETag: "test-etag", // Same as mock will return
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("performing reconciliation")
			// First reconcile adds finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile should skip fetch due to unchanged ETag
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Minute))

			By("verifying no new artifact was created")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			readyCondition := findCondition(updatedResource.Status.Conditions, ReadyCondition)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Message).To(Equal("ExternalSource is ready"))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle generator errors with retry logic", func() {
			resourceName := "test-generator-error"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("setting up mock generator to fail")
			mockFactory.CreateGeneratorFunc = func(generatorType string) (generator.SourceGenerator, error) {
				return &MockSourceGenerator{
					GenerateFunc: func(ctx context.Context, config generator.GeneratorConfig) (*generator.SourceData, error) {
						return nil, fmt.Errorf("network error")
					},
				}, nil
			}

			By("creating an ExternalSource resource")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("performing reconciliation")
			// First reconcile adds finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile should fail and schedule retry
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			Expect(result.RequeueAfter).To(BeNumerically("<=", 5*time.Minute))

			By("verifying error condition and retry count")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			readyCondition := findCondition(updatedResource.Status.Conditions, ReadyCondition)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal(FailedReason))
			Expect(readyCondition.Message).To(ContainSubstring("network error"))

			// Check retry count annotation
			Expect(updatedResource.Annotations).To(HaveKey("source.example.com/retry-count"))
			Expect(updatedResource.Annotations["source.example.com/retry-count"]).To(Equal("1"))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle transformation errors", func() {
			resourceName := "test-transform-error"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("setting up mock transformer to fail")
			mockTransformer.TransformFunc = func(ctx context.Context, input []byte, expression string) ([]byte, error) {
				return nil, fmt.Errorf("invalid CEL expression")
			}

			By("creating an ExternalSource resource with transformation")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
					Transform: &sourcev1alpha1.TransformSpec{
						Type:       "cel",
						Expression: "invalid.expression",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("performing reconciliation")
			// First reconcile adds finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile should fail during transformation
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			By("verifying transformation error condition")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			transformingCondition := findCondition(updatedResource.Status.Conditions, TransformingCondition)
			Expect(transformingCondition).NotTo(BeNil())
			Expect(transformingCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(transformingCondition.Reason).To(Equal(FailedReason))
			Expect(transformingCondition.Message).To(ContainSubstring("invalid CEL expression"))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle artifact storage errors", func() {
			resourceName := "test-storage-error"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("setting up mock artifact manager to fail storage")
			mockArtifactManager.StoreFunc = func(ctx context.Context, artifact *artifact.Artifact, source string) (string, error) {
				return "", fmt.Errorf("storage backend unavailable")
			}

			By("creating an ExternalSource resource")
			resource := &sourcev1alpha1.ExternalSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: sourcev1alpha1.ExternalSourceSpec{
					Interval: "5m",
					Generator: sourcev1alpha1.GeneratorSpec{
						Type: "http",
						HTTP: &sourcev1alpha1.HTTPGeneratorSpec{
							URL: "https://api.example.com/config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("performing reconciliation")
			// First reconcile adds finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Second reconcile should fail during storage
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			By("verifying storage error condition")
			var updatedResource sourcev1alpha1.ExternalSource
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedResource)).To(Succeed())

			storingCondition := findCondition(updatedResource.Status.Conditions, StoringCondition)
			Expect(storingCondition).NotTo(BeNil())
			Expect(storingCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(storingCondition.Reason).To(Equal(FailedReason))
			Expect(storingCondition.Message).To(ContainSubstring("storage backend unavailable"))

			By("cleaning up the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})

// Helper function to find a condition by type
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
