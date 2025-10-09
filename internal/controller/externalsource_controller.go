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
	"math"
	"math/rand"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sourcev1alpha1 "github.com/example/externalsource-controller/api/v1alpha1"
	"github.com/example/externalsource-controller/internal/artifact"
	"github.com/example/externalsource-controller/internal/generator"
	"github.com/example/externalsource-controller/internal/transformer"
)

// ExternalSourceReconciler reconciles a ExternalSource object
type ExternalSourceReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	GeneratorFactory  generator.SourceGeneratorFactory
	Transformer       transformer.Transformer
	ArtifactManager   artifact.ArtifactManager
}

const (
	// ExternalSourceFinalizer is the finalizer used by the ExternalSource controller
	ExternalSourceFinalizer = "source.example.com/externalsource-finalizer"
	
	// Retry configuration
	maxRetryAttempts = 10
	baseRetryDelay   = 1 * time.Second
	maxRetryDelay    = 5 * time.Minute
)

// Condition types for ExternalSource
const (
	// ReadyCondition indicates the overall status of the ExternalSource
	ReadyCondition = "Ready"

	// FetchingCondition indicates the source is currently being fetched
	FetchingCondition = "Fetching"

	// TransformingCondition indicates data is currently being transformed
	TransformingCondition = "Transforming"

	// StoringCondition indicates the artifact is currently being stored
	StoringCondition = "Storing"

	// StalledCondition indicates reconciliation has been stalled due to errors
	StalledCondition = "Stalled"
)

// Condition reasons
const (
	// SucceededReason indicates a successful operation
	SucceededReason = "Succeeded"

	// FailedReason indicates a failed operation
	FailedReason = "Failed"

	// ProgressingReason indicates an operation is in progress
	ProgressingReason = "Progressing"

	// SuspendedReason indicates the resource is suspended
	SuspendedReason = "Suspended"
)

// +kubebuilder:rbac:groups=source.example.com,resources=externalsources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.example.com,resources=externalsources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=source.example.com,resources=externalsources/finalizers,verbs=update
// +kubebuilder:rbac:groups=source.example.com,resources=externalartifacts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.example.com,resources=externalartifacts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ExternalSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the ExternalSource instance
	var externalSource sourcev1alpha1.ExternalSource
	if err := r.Get(ctx, req.NamespacedName, &externalSource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status conditions if not present
	if externalSource.Status.Conditions == nil {
		externalSource.Status.Conditions = []metav1.Condition{}
	}

	// Handle deletion
	if !externalSource.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &externalSource)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&externalSource, ExternalSourceFinalizer) {
		controllerutil.AddFinalizer(&externalSource, ExternalSourceFinalizer)
		if err := r.Update(ctx, &externalSource); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle suspension
	if externalSource.Spec.Suspend {
		log.Info("ExternalSource is suspended, skipping reconciliation")
		r.setCondition(&externalSource, ReadyCondition, metav1.ConditionFalse, SuspendedReason, "ExternalSource is suspended")
		if err := r.Status().Update(ctx, &externalSource); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Parse interval
	interval, err := time.ParseDuration(externalSource.Spec.Interval)
	if err != nil {
		log.Error(err, "Failed to parse interval")
		r.setCondition(&externalSource, ReadyCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Invalid interval: %v", err))
		if err := r.Status().Update(ctx, &externalSource); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure minimum interval of 1 minute
	if interval < time.Minute {
		interval = time.Minute
	}

	// Update observed generation
	externalSource.Status.ObservedGeneration = externalSource.Generation

	// Perform reconciliation
	_, err = r.reconcile(ctx, &externalSource)
	if err != nil {
		log.Error(err, "Reconciliation failed")
		
		// Determine if this is a transient error that should be retried
		retryDelay := r.calculateRetryDelay(&externalSource)
		if retryDelay > 0 {
			r.setCondition(&externalSource, ReadyCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Reconciliation failed, retrying in %v: %v", retryDelay, err.Error()))
			r.incrementRetryCount(&externalSource)
			
			if statusErr := r.Status().Update(ctx, &externalSource); statusErr != nil {
				log.Error(statusErr, "Failed to update status after reconciliation error")
			}
			
			return ctrl.Result{RequeueAfter: retryDelay}, nil
		} else {
			// Max retries exceeded or non-retryable error
			r.setCondition(&externalSource, StalledCondition, metav1.ConditionTrue, FailedReason, fmt.Sprintf("Max retries exceeded: %v", err.Error()))
			r.setCondition(&externalSource, ReadyCondition, metav1.ConditionFalse, FailedReason, err.Error())
			
			if statusErr := r.Status().Update(ctx, &externalSource); statusErr != nil {
				log.Error(statusErr, "Failed to update status after reconciliation error")
			}
			
			return ctrl.Result{RequeueAfter: interval}, nil
		}
	}
	
	// Clear retry count on successful reconciliation
	r.clearRetryCount(&externalSource)

	// Update status
	if err := r.Status().Update(ctx, &externalSource); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed", "requeue_after", interval)
	
	return ctrl.Result{RequeueAfter: interval}, nil
}

// reconcile performs the main reconciliation logic
func (r *ExternalSourceReconciler) reconcile(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Create generator configuration from ExternalSource spec
	generatorConfig, err := r.createGeneratorConfig(externalSource)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create generator config: %w", err)
	}

	// Create source generator
	sourceGenerator, err := r.GeneratorFactory.CreateGenerator(externalSource.Spec.Generator.Type)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create source generator: %w", err)
	}

	// Check if we can use conditional fetching
	shouldFetch := true
	if sourceGenerator.SupportsConditionalFetch() && externalSource.Status.LastHandledETag != "" {
		r.setCondition(externalSource, FetchingCondition, metav1.ConditionTrue, ProgressingReason, "Checking for updates")
		
		currentETag, err := sourceGenerator.GetLastModified(ctx, *generatorConfig)
		if err != nil {
			log.Info("Failed to get last modified, proceeding with full fetch", "error", err)
		} else if currentETag != "" && currentETag == externalSource.Status.LastHandledETag {
			log.Info("No changes detected, skipping fetch", "etag", currentETag)
			r.setCondition(externalSource, FetchingCondition, metav1.ConditionFalse, SucceededReason, "No changes detected")
			r.setCondition(externalSource, ReadyCondition, metav1.ConditionTrue, SucceededReason, "ExternalSource is ready")
			return ctrl.Result{}, nil
		}
	}

	if shouldFetch {
		// Fetch data from source
		r.setCondition(externalSource, FetchingCondition, metav1.ConditionTrue, ProgressingReason, "Fetching data from external source")
		
		sourceData, err := sourceGenerator.Generate(ctx, *generatorConfig)
		if err != nil {
			r.setCondition(externalSource, FetchingCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Failed to fetch data: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to generate source data: %w", err)
		}

		r.setCondition(externalSource, FetchingCondition, metav1.ConditionFalse, SucceededReason, "Successfully fetched data")

		// Transform data if transformation is specified
		transformedData := sourceData.Data
		if externalSource.Spec.Transform != nil {
			r.setCondition(externalSource, TransformingCondition, metav1.ConditionTrue, ProgressingReason, "Transforming data")
			
			transformedData, err = r.Transformer.Transform(ctx, sourceData.Data, externalSource.Spec.Transform.Expression)
			if err != nil {
				r.setCondition(externalSource, TransformingCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Failed to transform data: %v", err))
				return ctrl.Result{}, fmt.Errorf("failed to transform data: %w", err)
			}
			
			r.setCondition(externalSource, TransformingCondition, metav1.ConditionFalse, SucceededReason, "Successfully transformed data")
		}

		// Package and store artifact
		r.setCondition(externalSource, StoringCondition, metav1.ConditionTrue, ProgressingReason, "Packaging and storing artifact")
		
		destinationPath := externalSource.Spec.DestinationPath
		if destinationPath == "" {
			destinationPath = "data"
		}

		packagedArtifact, err := r.ArtifactManager.Package(ctx, transformedData, destinationPath)
		if err != nil {
			r.setCondition(externalSource, StoringCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Failed to package artifact: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to package artifact: %w", err)
		}

		// Store artifact and get URL
		sourceKey := fmt.Sprintf("%s/%s", externalSource.Namespace, externalSource.Name)
		artifactURL, err := r.ArtifactManager.Store(ctx, packagedArtifact, sourceKey)
		if err != nil {
			r.setCondition(externalSource, StoringCondition, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Failed to store artifact: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to store artifact: %w", err)
		}

		r.setCondition(externalSource, StoringCondition, metav1.ConditionFalse, SucceededReason, "Successfully stored artifact")

		// Update status with new artifact information
		externalSource.Status.Artifact = &sourcev1alpha1.ArtifactMetadata{
			URL:            artifactURL,
			Revision:       packagedArtifact.Revision,
			LastUpdateTime: metav1.Now(),
			Metadata:       packagedArtifact.Metadata,
		}

		// Update last handled ETag
		if sourceData.LastModified != "" {
			externalSource.Status.LastHandledETag = sourceData.LastModified
		}

		// Clean up old artifacts
		if err := r.ArtifactManager.Cleanup(ctx, sourceKey, packagedArtifact.Revision); err != nil {
			log.Error(err, "Failed to cleanup old artifacts", "source", sourceKey, "keepRevision", packagedArtifact.Revision)
			// Don't fail reconciliation for cleanup errors
		}

		// Create or update ExternalArtifact child resource
		if err := r.reconcileExternalArtifact(ctx, externalSource, artifactURL, packagedArtifact.Revision, packagedArtifact.Metadata); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile ExternalArtifact: %w", err)
		}

		log.Info("Successfully processed external source", "url", artifactURL, "revision", packagedArtifact.Revision)
	}

	// Set overall ready condition
	r.setCondition(externalSource, ReadyCondition, metav1.ConditionTrue, SucceededReason, "ExternalSource is ready")

	return ctrl.Result{}, nil
}

// createGeneratorConfig creates a generator configuration from the ExternalSource spec
func (r *ExternalSourceReconciler) createGeneratorConfig(externalSource *sourcev1alpha1.ExternalSource) (*generator.GeneratorConfig, error) {
	config := &generator.GeneratorConfig{
		Type:   externalSource.Spec.Generator.Type,
		Config: make(map[string]interface{}),
	}

	// Add namespace for secret resolution
	config.Config["namespace"] = externalSource.Namespace

	// Configure based on generator type
	switch externalSource.Spec.Generator.Type {
	case "http":
		if externalSource.Spec.Generator.HTTP == nil {
			return nil, fmt.Errorf("HTTP configuration is required for HTTP generator")
		}

		httpSpec := externalSource.Spec.Generator.HTTP
		config.Config["url"] = httpSpec.URL
		
		if httpSpec.Method != "" {
			config.Config["method"] = httpSpec.Method
		}
		
		if httpSpec.InsecureSkipVerify {
			config.Config["insecureSkipVerify"] = true
		}
		
		if httpSpec.HeadersSecretRef != nil && httpSpec.HeadersSecretRef.Name != "" {
			config.Config["headersSecretName"] = httpSpec.HeadersSecretRef.Name
		}
		
		if httpSpec.CABundleSecretRef != nil && httpSpec.CABundleSecretRef.Name != "" {
			config.Config["caBundleSecretName"] = httpSpec.CABundleSecretRef.Name
			if httpSpec.CABundleSecretRef.Key != "" {
				config.Config["caBundleSecretKey"] = httpSpec.CABundleSecretRef.Key
			}
		}

	default:
		return nil, fmt.Errorf("unsupported generator type: %s", externalSource.Spec.Generator.Type)
	}

	return config, nil
}

// reconcileExternalArtifact creates or updates the ExternalArtifact child resource
func (r *ExternalSourceReconciler) reconcileExternalArtifact(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource, artifactURL, revision string, metadata map[string]string) error {
	log := logf.FromContext(ctx)

	// Create ExternalArtifact name based on ExternalSource name
	artifactName := externalSource.Name

	// Check if ExternalArtifact already exists
	existingArtifact := &sourcev1alpha1.ExternalArtifact{}
	artifactKey := client.ObjectKey{
		Namespace: externalSource.Namespace,
		Name:      artifactName,
	}

	err := r.Get(ctx, artifactKey, existingArtifact)
	if err != nil && client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ExternalArtifact: %w", err)
	}

	// Create new ExternalArtifact if it doesn't exist
	if client.IgnoreNotFound(err) != nil {
		newArtifact := &sourcev1alpha1.ExternalArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      artifactName,
				Namespace: externalSource.Namespace,
			},
			Spec: sourcev1alpha1.ExternalArtifactSpec{
				URL:      artifactURL,
				Revision: revision,
				Metadata: metadata,
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(externalSource, newArtifact, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		log.Info("Creating ExternalArtifact", "name", artifactName, "url", artifactURL, "revision", revision)
		if err := r.Create(ctx, newArtifact); err != nil {
			return fmt.Errorf("failed to create ExternalArtifact: %w", err)
		}

		return nil
	}

	// Update existing ExternalArtifact if needed
	needsUpdate := false
	if existingArtifact.Spec.URL != artifactURL {
		existingArtifact.Spec.URL = artifactURL
		needsUpdate = true
	}
	if existingArtifact.Spec.Revision != revision {
		existingArtifact.Spec.Revision = revision
		needsUpdate = true
	}
	if !mapsEqual(existingArtifact.Spec.Metadata, metadata) {
		existingArtifact.Spec.Metadata = metadata
		needsUpdate = true
	}

	if needsUpdate {
		log.Info("Updating ExternalArtifact", "name", artifactName, "url", artifactURL, "revision", revision)
		if err := r.Update(ctx, existingArtifact); err != nil {
			return fmt.Errorf("failed to update ExternalArtifact: %w", err)
		}
	}

	return nil
}

// reconcileDelete handles the deletion of an ExternalSource
func (r *ExternalSourceReconciler) reconcileDelete(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Cleaning up ExternalSource")

	// Clean up artifacts from storage
	if externalSource.Status.Artifact != nil {
		sourceKey := fmt.Sprintf("%s/%s", externalSource.Namespace, externalSource.Name)
		if err := r.ArtifactManager.Cleanup(ctx, sourceKey, ""); err != nil {
			log.Error(err, "Failed to cleanup artifacts from storage", "source", sourceKey)
			// Don't fail deletion for cleanup errors, just log them
		}
	}

	// Clean up child ExternalArtifact resources (handled automatically by owner references)
	// The Kubernetes garbage collector will delete the ExternalArtifact when the ExternalSource is deleted

	// Remove finalizer
	controllerutil.RemoveFinalizer(externalSource, ExternalSourceFinalizer)
	if err := r.Update(ctx, externalSource); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setCondition sets a condition on the ExternalSource status
func (r *ExternalSourceReconciler) setCondition(externalSource *sourcev1alpha1.ExternalSource, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: externalSource.Generation,
	}

	apimeta.SetStatusCondition(&externalSource.Status.Conditions, condition)
}

// calculateRetryDelay calculates the retry delay using exponential backoff with jitter
func (r *ExternalSourceReconciler) calculateRetryDelay(externalSource *sourcev1alpha1.ExternalSource) time.Duration {
	retryCount := r.getRetryCount(externalSource)
	
	if retryCount >= maxRetryAttempts {
		return 0 // No more retries
	}
	
	// Exponential backoff: baseDelay * 2^retryCount
	delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(retryCount)))
	
	// Cap at maximum delay
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	
	// Add jitter (±25% of the delay)
	jitter := time.Duration(float64(delay) * 0.25 * (2*rand.Float64() - 1))
	delay += jitter
	
	// Ensure minimum delay
	if delay < baseRetryDelay {
		delay = baseRetryDelay
	}
	
	return delay
}

// getRetryCount gets the current retry count from annotations
func (r *ExternalSourceReconciler) getRetryCount(externalSource *sourcev1alpha1.ExternalSource) int {
	if externalSource.Annotations == nil {
		return 0
	}
	
	retryCountStr, exists := externalSource.Annotations["source.example.com/retry-count"]
	if !exists {
		return 0
	}
	
	var retryCount int
	if _, err := fmt.Sscanf(retryCountStr, "%d", &retryCount); err != nil {
		return 0
	}
	
	return retryCount
}

// incrementRetryCount increments the retry count in annotations
func (r *ExternalSourceReconciler) incrementRetryCount(externalSource *sourcev1alpha1.ExternalSource) {
	if externalSource.Annotations == nil {
		externalSource.Annotations = make(map[string]string)
	}
	
	retryCount := r.getRetryCount(externalSource)
	externalSource.Annotations["source.example.com/retry-count"] = fmt.Sprintf("%d", retryCount+1)
}

// clearRetryCount clears the retry count from annotations
func (r *ExternalSourceReconciler) clearRetryCount(externalSource *sourcev1alpha1.ExternalSource) {
	if externalSource.Annotations != nil {
		delete(externalSource.Annotations, "source.example.com/retry-count")
		
		// Remove stalled condition if it exists
		apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, StalledCondition)
	}
}

// mapsEqual compares two string maps for equality
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Register built-in generators
	if err := r.GeneratorFactory.RegisterGenerator("http", func() generator.SourceGenerator {
		return generator.NewHTTPGenerator(r.Client)
	}); err != nil {
		return fmt.Errorf("failed to register HTTP generator: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sourcev1alpha1.ExternalSource{}).
		Owns(&sourcev1alpha1.ExternalArtifact{}).
		Named("externalsource").
		Complete(r)
}
