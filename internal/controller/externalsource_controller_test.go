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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sourcev1alpha1 "github.com/example/externalsource-controller/api/v1alpha1"
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
			Expect(result.Requeue).To(BeTrue())

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
			Expect(result.Requeue).To(BeTrue())

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

// Helper function to find a condition by type
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
