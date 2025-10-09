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
	"github.com/example/externalsource-controller/internal/metrics"
	"github.com/example/externalsource-controller/internal/transformer"
)

// ExternalSourceReconciler reconciles a ExternalSource object
type ExternalSourceReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GeneratorFactory generator.SourceGeneratorFactory
	Transformer      transformer.Transformer
	ArtifactManager  artifact.ArtifactManager
	MetricsRecorder  metrics.MetricsRecorder
}

const (
	// ExternalSourceFinalizer is the finalizer used by the ExternalSource controller
	ExternalSourceFinalizer = "source.example.com/externalsource-finalizer"

	// Retry configuration
	maxRetryAttempts = 10
	baseRetryDelay   = 1 * time.Second
	maxRetryDelay    = 5 * time.Minute
	jitterFactor     = 0.25 // ±25% jitter

	// Annotation keys for retry tracking
	retryCountAnnotation   = "source.example.com/retry-count"
	lastFailureAnnotation  = "source.example.com/last-failure"
	backoffStartAnnotation = "source.example.com/backoff-start"
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
	startTime := time.Now()

	// Track active reconciliations
	if r.MetricsRecorder != nil {
		r.MetricsRecorder.IncActiveReconciliations(req.Namespace, req.Name)
		defer r.MetricsRecorder.DecActiveReconciliations(req.Namespace, req.Name)
	}

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
		r.setReadyCondition(&externalSource, metav1.ConditionFalse, SuspendedReason, "ExternalSource is suspended")
		if err := r.Status().Update(ctx, &externalSource); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Parse interval
	interval, err := time.ParseDuration(externalSource.Spec.Interval)
	if err != nil {
		log.Error(err, "Failed to parse interval")
		r.setReadyCondition(&externalSource, metav1.ConditionFalse, FailedReason, fmt.Sprintf("Invalid interval: %v", err))
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

	// Check for controller restart recovery
	if r.needsRecovery(&externalSource) {
		log.Info("Detected controller restart, performing recovery",
			"last_artifact", externalSource.Status.Artifact != nil)
		r.performRecovery(ctx, &externalSource)
	}

	// Store previous artifact info for graceful degradation
	previousArtifact := externalSource.Status.Artifact

	// Perform reconciliation
	_, err = r.reconcile(ctx, &externalSource)

	// Record reconciliation metrics
	sourceType := externalSource.Spec.Generator.Type
	reconciliationSuccess := err == nil
	reconciliationDuration := time.Since(startTime)

	if r.MetricsRecorder != nil {
		r.MetricsRecorder.RecordReconciliation(
			externalSource.Namespace,
			externalSource.Name,
			sourceType,
			reconciliationSuccess,
			reconciliationDuration,
		)
	}

	if err != nil {
		log.Error(err, "Reconciliation failed")

		// Reset retry count if spec has changed
		if r.shouldResetRetryCount(&externalSource) {
			r.clearRetryCount(&externalSource)
		}

		// Determine if this is a transient error that should be retried
		retryDelay := r.calculateRetryDelay(&externalSource, err)
		errorType := r.classifyError(err)

		if retryDelay > 0 && errorType == TransientError {
			retryCount := r.getRetryCount(&externalSource)
			backoffDuration := r.getBackoffDuration(&externalSource)

			// Maintain last successful artifact during transient failures (graceful degradation)
			if previousArtifact != nil {
				externalSource.Status.Artifact = previousArtifact
				log.Info("Maintaining last successful artifact during transient failure",
					"artifact_url", previousArtifact.URL, "revision", previousArtifact.Revision)
			}

			r.setReadyCondition(&externalSource, metav1.ConditionFalse, FailedReason,
				fmt.Sprintf("Reconciliation failed (attempt %d/%d, in backoff for %v), retrying in %v. Last successful artifact maintained: %v",
					retryCount+1, maxRetryAttempts, backoffDuration.Truncate(time.Second), retryDelay.Truncate(time.Second), err.Error()))

			r.incrementRetryCount(&externalSource, err)

			if statusErr := r.Status().Update(ctx, &externalSource); statusErr != nil {
				log.Error(statusErr, "Failed to update status after reconciliation error")
			}

			return ctrl.Result{RequeueAfter: retryDelay}, nil
		} else {
			// Max retries exceeded, permanent error, or configuration error
			var reason, message string

			switch errorType {
			case ConfigurationError:
				reason = "ConfigurationError"
				message = fmt.Sprintf("Configuration error (will not retry until spec changes): %v", err.Error())
			case PermanentError:
				reason = "PermanentError"
				message = fmt.Sprintf("Permanent error (will not retry): %v", err.Error())
			default:
				reason = "MaxRetriesExceeded"
				message = fmt.Sprintf("Max retries exceeded (%d attempts): %v", maxRetryAttempts, err.Error())
				r.setCondition(&externalSource, StalledCondition, metav1.ConditionTrue, reason, message)
			}

			r.setReadyCondition(&externalSource, metav1.ConditionFalse, reason, message)

			if statusErr := r.Status().Update(ctx, &externalSource); statusErr != nil {
				log.Error(statusErr, "Failed to update status after reconciliation error")
			}

			// For configuration errors, don't requeue until spec changes
			if errorType == ConfigurationError {
				return ctrl.Result{}, nil
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
		r.setProgressCondition(externalSource, FetchingCondition, true, ProgressingReason, "Checking for updates")

		currentETag, err := sourceGenerator.GetLastModified(ctx, *generatorConfig)
		if err != nil {
			log.Info("Failed to get last modified, proceeding with full fetch", "error", err)
		} else if currentETag != "" && currentETag == externalSource.Status.LastHandledETag {
			log.Info("No changes detected, skipping fetch", "etag", currentETag)
			r.setProgressCondition(externalSource, FetchingCondition, false, SucceededReason, "No changes detected")
			r.setReadyCondition(externalSource, metav1.ConditionTrue, SucceededReason, "ExternalSource is ready")
			return ctrl.Result{}, nil
		}
	}

	if shouldFetch {
		// Fetch data from source
		r.setProgressCondition(externalSource, FetchingCondition, true, ProgressingReason, "Fetching data from external source")

		fetchStartTime := time.Now()
		sourceData, err := sourceGenerator.Generate(ctx, *generatorConfig)
		fetchDuration := time.Since(fetchStartTime)

		// Record source request metrics
		if r.MetricsRecorder != nil {
			r.MetricsRecorder.RecordSourceRequest(externalSource.Spec.Generator.Type, err == nil, fetchDuration)
		}

		if err != nil {
			r.setProgressCondition(externalSource, FetchingCondition, false, FailedReason, fmt.Sprintf("Failed to fetch data: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to generate source data: %w", err)
		}

		r.setProgressCondition(externalSource, FetchingCondition, false, SucceededReason, "Successfully fetched data")

		// Transform data if transformation is specified
		transformedData := sourceData.Data
		if externalSource.Spec.Transform != nil {
			r.setProgressCondition(externalSource, TransformingCondition, true, ProgressingReason, "Transforming data")

			transformStartTime := time.Now()
			transformedData, err = r.Transformer.Transform(ctx, sourceData.Data, externalSource.Spec.Transform.Expression)
			transformDuration := time.Since(transformStartTime)

			// Record transformation metrics
			if r.MetricsRecorder != nil {
				r.MetricsRecorder.RecordTransformation(err == nil, transformDuration)
			}

			if err != nil {
				r.setProgressCondition(externalSource, TransformingCondition, false, FailedReason, fmt.Sprintf("Failed to transform data: %v", err))
				return ctrl.Result{}, fmt.Errorf("failed to transform data: %w", err)
			}

			r.setProgressCondition(externalSource, TransformingCondition, false, SucceededReason, "Successfully transformed data")
		}

		// Package and store artifact
		r.setProgressCondition(externalSource, StoringCondition, true, ProgressingReason, "Packaging and storing artifact")

		destinationPath := externalSource.Spec.DestinationPath
		if destinationPath == "" {
			destinationPath = "data"
		}

		// Package artifact
		packageStartTime := time.Now()
		packagedArtifact, err := r.ArtifactManager.Package(ctx, transformedData, destinationPath)
		packageDuration := time.Since(packageStartTime)

		// Record packaging metrics
		if r.MetricsRecorder != nil {
			r.MetricsRecorder.RecordArtifactOperation("package", err == nil, packageDuration)
		}

		if err != nil {
			r.setProgressCondition(externalSource, StoringCondition, false, FailedReason, fmt.Sprintf("Failed to package artifact: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to package artifact: %w", err)
		}

		// Store artifact and get URL
		sourceKey := fmt.Sprintf("%s/%s", externalSource.Namespace, externalSource.Name)
		storeStartTime := time.Now()
		artifactURL, err := r.ArtifactManager.Store(ctx, packagedArtifact, sourceKey)
		storeDuration := time.Since(storeStartTime)

		// Record storage metrics
		if r.MetricsRecorder != nil {
			r.MetricsRecorder.RecordArtifactOperation("store", err == nil, storeDuration)
		}

		if err != nil {
			r.setProgressCondition(externalSource, StoringCondition, false, FailedReason, fmt.Sprintf("Failed to store artifact: %v", err))
			return ctrl.Result{}, fmt.Errorf("failed to store artifact: %w", err)
		}

		r.setProgressCondition(externalSource, StoringCondition, false, SucceededReason, "Successfully stored artifact")

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

	// Clear progress conditions and set overall ready condition
	r.clearProgressConditions(externalSource)
	r.setReadyCondition(externalSource, metav1.ConditionTrue, SucceededReason, "ExternalSource is ready")

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

	// Create new ExternalArtifact if it doesn't exist (when err is NotFound)
	if err != nil && client.IgnoreNotFound(err) == nil {
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

// setReadyCondition is a helper to set the Ready condition with proper status
func (r *ExternalSourceReconciler) setReadyCondition(externalSource *sourcev1alpha1.ExternalSource, status metav1.ConditionStatus, reason, message string) {
	r.setCondition(externalSource, ReadyCondition, status, reason, message)
}

// setProgressCondition sets a progress condition (Fetching, Transforming, Storing)
func (r *ExternalSourceReconciler) setProgressCondition(externalSource *sourcev1alpha1.ExternalSource, conditionType string, inProgress bool, reason, message string) {
	status := metav1.ConditionFalse
	if inProgress {
		status = metav1.ConditionTrue
	}
	r.setCondition(externalSource, conditionType, status, reason, message)
}

// clearProgressConditions clears all progress conditions when reconciliation is complete
func (r *ExternalSourceReconciler) clearProgressConditions(externalSource *sourcev1alpha1.ExternalSource) {
	// Clear progress conditions that should not persist after reconciliation
	apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, FetchingCondition)
	apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, TransformingCondition)
	apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, StoringCondition)
}

// hasCondition checks if a condition exists with the given type and status
func (r *ExternalSourceReconciler) hasCondition(externalSource *sourcev1alpha1.ExternalSource, conditionType string, status metav1.ConditionStatus) bool {
	condition := apimeta.FindStatusCondition(externalSource.Status.Conditions, conditionType)
	return condition != nil && condition.Status == status
}

// getConditionMessage gets the message for a specific condition type
func (r *ExternalSourceReconciler) getConditionMessage(externalSource *sourcev1alpha1.ExternalSource, conditionType string) string {
	condition := apimeta.FindStatusCondition(externalSource.Status.Conditions, conditionType)
	if condition != nil {
		return condition.Message
	}
	return ""
}

// ErrorType represents the type of error for retry classification
type ErrorType int

const (
	TransientError ErrorType = iota
	PermanentError
	ConfigurationError
)

// classifyError determines if an error is transient and should be retried
func (r *ExternalSourceReconciler) classifyError(err error) ErrorType {
	if err == nil {
		return TransientError // Should not happen, but safe default
	}

	errStr := err.Error()

	// Configuration errors - don't retry until spec changes
	configErrors := []string{
		"invalid interval",
		"unsupported generator type",
		"configuration is required",
		"invalid URL",
		"invalid CEL expression",
	}

	for _, configErr := range configErrors {
		if contains(errStr, configErr) {
			return ConfigurationError
		}
	}

	// Permanent errors - don't retry
	permanentErrors := []string{
		"404",
		"401",
		"403",
		"not found",
		"unauthorized",
		"forbidden",
	}

	for _, permErr := range permanentErrors {
		if contains(errStr, permErr) {
			return PermanentError
		}
	}

	// Default to transient for network errors, timeouts, etc.
	return TransientError
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

// findSubstring performs a simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// calculateRetryDelay calculates the retry delay using exponential backoff with jitter
func (r *ExternalSourceReconciler) calculateRetryDelay(externalSource *sourcev1alpha1.ExternalSource, err error) time.Duration {
	// Classify the error
	errorType := r.classifyError(err)

	// Don't retry configuration or permanent errors
	if errorType == ConfigurationError || errorType == PermanentError {
		return 0
	}

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

	// Add jitter to prevent thundering herd
	// Use a deterministic seed based on the resource to ensure consistent jitter
	seed := int64(0)
	for _, b := range []byte(externalSource.Namespace + "/" + externalSource.Name) {
		seed = seed*31 + int64(b)
	}
	rng := rand.New(rand.NewSource(seed + int64(retryCount)))

	jitter := time.Duration(float64(delay) * jitterFactor * (2*rng.Float64() - 1))
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

	retryCountStr, exists := externalSource.Annotations[retryCountAnnotation]
	if !exists {
		return 0
	}

	var retryCount int
	if _, err := fmt.Sscanf(retryCountStr, "%d", &retryCount); err != nil {
		return 0
	}

	return retryCount
}

// incrementRetryCount increments the retry count and tracks failure information
func (r *ExternalSourceReconciler) incrementRetryCount(externalSource *sourcev1alpha1.ExternalSource, err error) {
	if externalSource.Annotations == nil {
		externalSource.Annotations = make(map[string]string)
	}

	retryCount := r.getRetryCount(externalSource)
	newRetryCount := retryCount + 1

	externalSource.Annotations[retryCountAnnotation] = fmt.Sprintf("%d", newRetryCount)
	externalSource.Annotations[lastFailureAnnotation] = err.Error()

	// Set backoff start time on first failure
	if retryCount == 0 {
		externalSource.Annotations[backoffStartAnnotation] = time.Now().Format(time.RFC3339)
	}
}

// clearRetryCount clears all retry-related annotations and conditions
func (r *ExternalSourceReconciler) clearRetryCount(externalSource *sourcev1alpha1.ExternalSource) {
	if externalSource.Annotations != nil {
		delete(externalSource.Annotations, retryCountAnnotation)
		delete(externalSource.Annotations, lastFailureAnnotation)
		delete(externalSource.Annotations, backoffStartAnnotation)

		// Remove stalled condition if it exists
		apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, StalledCondition)
	}
}

// getBackoffDuration returns how long the resource has been in backoff
func (r *ExternalSourceReconciler) getBackoffDuration(externalSource *sourcev1alpha1.ExternalSource) time.Duration {
	if externalSource.Annotations == nil {
		return 0
	}

	backoffStartStr, exists := externalSource.Annotations[backoffStartAnnotation]
	if !exists {
		return 0
	}

	backoffStart, err := time.Parse(time.RFC3339, backoffStartStr)
	if err != nil {
		return 0
	}

	return time.Since(backoffStart)
}

// shouldResetRetryCount determines if retry count should be reset based on spec changes
func (r *ExternalSourceReconciler) shouldResetRetryCount(externalSource *sourcev1alpha1.ExternalSource) bool {
	// Reset retry count if the spec has changed (observedGeneration mismatch)
	return externalSource.Status.ObservedGeneration != externalSource.Generation
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

// needsRecovery determines if the controller needs to perform recovery after restart
func (r *ExternalSourceReconciler) needsRecovery(externalSource *sourcev1alpha1.ExternalSource) bool {
	// Check if there are any in-progress conditions that suggest the controller was interrupted
	inProgressConditions := []string{FetchingCondition, TransformingCondition, StoringCondition}

	for _, conditionType := range inProgressConditions {
		if r.hasCondition(externalSource, conditionType, metav1.ConditionTrue) {
			return true
		}
	}

	// Check if there's a stalled condition that might need recovery
	if r.hasCondition(externalSource, StalledCondition, metav1.ConditionTrue) {
		// If we have a successful artifact but are marked as stalled, we might need recovery
		return externalSource.Status.Artifact != nil
	}

	return false
}

// performRecovery handles controller restart recovery
func (r *ExternalSourceReconciler) performRecovery(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource) {
	log := logf.FromContext(ctx)

	// Clear any in-progress conditions from before the restart
	r.clearProgressConditions(externalSource)

	// If we have a last successful artifact, ensure the ExternalArtifact child resource exists
	if externalSource.Status.Artifact != nil {
		statusArtifact := externalSource.Status.Artifact
		log.Info("Recovering with last successful artifact",
			"url", statusArtifact.URL, "revision", statusArtifact.Revision)

		// Ensure ExternalArtifact child resource is properly created/updated
		if err := r.reconcileExternalArtifact(ctx, externalSource,
			statusArtifact.URL, statusArtifact.Revision, statusArtifact.Metadata); err != nil {
			log.Error(err, "Failed to recover ExternalArtifact during controller restart")
		}

		// Set ready condition to indicate we have a working artifact
		r.setReadyCondition(externalSource, metav1.ConditionTrue, SucceededReason,
			"Recovered from controller restart with last successful artifact")
	} else {
		// No previous artifact, start fresh
		log.Info("No previous artifact found, starting fresh reconciliation")
		r.setReadyCondition(externalSource, metav1.ConditionFalse, ProgressingReason,
			"Starting fresh reconciliation after controller restart")
	}

	// Clear any stalled condition since we're actively recovering
	apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, StalledCondition)
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
