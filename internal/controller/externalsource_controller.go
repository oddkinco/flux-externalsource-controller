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

// Package controller implements the Kubernetes controller for ExternalSource resources.
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

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1alpha1 "github.com/oddkinco/flux-externalsource-controller/api/v1alpha1"
	"github.com/oddkinco/flux-externalsource-controller/internal/artifact"
	"github.com/oddkinco/flux-externalsource-controller/internal/config"
	"github.com/oddkinco/flux-externalsource-controller/internal/generator"
	"github.com/oddkinco/flux-externalsource-controller/internal/hooks"
	"github.com/oddkinco/flux-externalsource-controller/internal/metrics"
	"github.com/oddkinco/flux-externalsource-controller/internal/storage"
)

// ExternalSourceReconciler reconciles a ExternalSource object
type ExternalSourceReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GeneratorFactory generator.SourceGeneratorFactory
	HookExecutor     hooks.HookExecutor
	ArtifactManager  artifact.ArtifactManager
	MetricsRecorder  metrics.MetricsRecorder
	Config           *config.Config
	StorageBackend   storage.StorageBackend // Optional: can be set externally to share with artifact server
}

const (
	// ExternalSourceFinalizer is the finalizer used by the ExternalSource controller
	ExternalSourceFinalizer = "source.flux.oddkin.co/externalsource-finalizer"

	// Annotation keys for retry tracking
	retryCountAnnotation   = "source.flux.oddkin.co/retry-count"
	lastFailureAnnotation  = "source.flux.oddkin.co/last-failure"
	backoffStartAnnotation = "source.flux.oddkin.co/backoff-start"
)

// Condition types for ExternalSource
const (
	// ReadyCondition indicates the overall status of the ExternalSource
	ReadyCondition = "Ready"

	// FetchingCondition indicates the source is currently being fetched
	FetchingCondition = "Fetching"

	// ExecutingHooksCondition indicates hooks are currently being executed
	ExecutingHooksCondition = "ExecutingHooks"

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

// +kubebuilder:rbac:groups=source.flux.oddkin.co,resources=externalsources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.flux.oddkin.co,resources=externalsources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=source.flux.oddkin.co,resources=externalsources/finalizers,verbs=update
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=externalartifacts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=externalartifacts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
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
					retryCount+1, r.Config.Retry.MaxAttempts, backoffDuration.Truncate(time.Second), retryDelay.Truncate(time.Second), err.Error()))

			r.incrementRetryCount(&externalSource, err)

			if statusErr := r.Status().Update(ctx, &externalSource); statusErr != nil {
				log.Error(statusErr, "Failed to update status after reconciliation error")
			}

			return ctrl.Result{RequeueAfter: retryDelay}, nil
		} else { //nolint:revive // Complex error handling logic is clearer with explicit else
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
				message = fmt.Sprintf("Max retries exceeded (%d attempts): %v", r.Config.Retry.MaxAttempts, err.Error())
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
//
//nolint:unparam // ctrl.Result is always nil but required by interface contract for future extensibility
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

		// Execute post-request hooks if specified
		processedData := sourceData.Data
		if externalSource.Spec.Hooks != nil && len(externalSource.Spec.Hooks.PostRequest) > 0 {
			r.setProgressCondition(externalSource, ExecutingHooksCondition, true, ProgressingReason, "Executing post-request hooks")

			var hookErr error
			processedData, hookErr = r.executeHooks(ctx, externalSource, processedData, externalSource.Spec.Hooks.PostRequest)
			if hookErr != nil {
				r.setProgressCondition(externalSource, ExecutingHooksCondition, false, FailedReason, fmt.Sprintf("Failed to execute hooks: %v", hookErr))
				return ctrl.Result{}, fmt.Errorf("failed to execute post-request hooks: %w", hookErr)
			}

			r.setProgressCondition(externalSource, ExecutingHooksCondition, false, SucceededReason, "Successfully executed post-request hooks")
		}

		// Package and store artifact
		r.setProgressCondition(externalSource, StoringCondition, true, ProgressingReason, "Packaging and storing artifact")

		destinationPath := externalSource.Spec.DestinationPath
		if destinationPath == "" {
			destinationPath = "data"
		}

		// Package artifact
		packageStartTime := time.Now()
		packagedArtifact, err := r.ArtifactManager.Package(ctx, processedData, destinationPath)
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
	genConfig := &generator.GeneratorConfig{
		Type:   externalSource.Spec.Generator.Type,
		Config: make(map[string]interface{}),
	}

	// Add namespace for secret resolution
	genConfig.Config["namespace"] = externalSource.Namespace

	// Configure based on generator type
	switch externalSource.Spec.Generator.Type {
	case "http":
		if externalSource.Spec.Generator.HTTP == nil {
			return nil, fmt.Errorf("HTTP configuration is required for HTTP generator")
		}

		httpSpec := externalSource.Spec.Generator.HTTP
		genConfig.Config["url"] = httpSpec.URL

		if httpSpec.Method != "" {
			genConfig.Config["method"] = httpSpec.Method
		}

		if httpSpec.InsecureSkipVerify {
			genConfig.Config["insecureSkipVerify"] = true
		}

		if httpSpec.HeadersSecretRef != nil && httpSpec.HeadersSecretRef.Name != "" {
			genConfig.Config["headersSecretName"] = httpSpec.HeadersSecretRef.Name
		}

		if httpSpec.CABundleSecretRef != nil && httpSpec.CABundleSecretRef.Name != "" {
			genConfig.Config["caBundleSecretName"] = httpSpec.CABundleSecretRef.Name
			if httpSpec.CABundleSecretRef.Key != "" {
				genConfig.Config["caBundleSecretKey"] = httpSpec.CABundleSecretRef.Key
			}
		}

	default:
		return nil, fmt.Errorf("unsupported generator type: %s", externalSource.Spec.Generator.Type)
	}

	return genConfig, nil
}

// reconcileExternalArtifact creates or updates the ExternalArtifact child resource
func (r *ExternalSourceReconciler) reconcileExternalArtifact(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource, artifactURL, revision string, metadata map[string]string) error {
	log := logf.FromContext(ctx)

	// Create ExternalArtifact name based on ExternalSource name
	artifactName := externalSource.Name

	// Check if ExternalArtifact already exists
	existingArtifact := &sourcev1.ExternalArtifact{}
	artifactKey := client.ObjectKey{
		Namespace: externalSource.Namespace,
		Name:      artifactName,
	}

	err := r.Get(ctx, artifactKey, existingArtifact)
	if err != nil && client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ExternalArtifact: %w", err)
	}

	// Prepare artifact metadata
	artifactMetadata := make(map[string]string)
	for k, v := range metadata {
		artifactMetadata[k] = v
	}

	// Create artifact object for status
	artifact := &fluxmeta.Artifact{
		URL:            artifactURL,
		Path:           artifactURL, // Path can be the same as URL for external artifacts
		Revision:       revision,
		Digest:         "sha256:" + revision, // Format as sha256 digest
		LastUpdateTime: metav1.Now(),
		Metadata:       artifactMetadata,
	}

	// Create new ExternalArtifact if it doesn't exist (when err is NotFound)
	if err != nil && client.IgnoreNotFound(err) == nil {
		newArtifact := &sourcev1.ExternalArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      artifactName,
				Namespace: externalSource.Namespace,
			},
			Spec: sourcev1.ExternalArtifactSpec{
				SourceRef: &fluxmeta.NamespacedObjectKindReference{
					APIVersion: sourcev1alpha1.GroupVersion.String(),
					Kind:       "ExternalSource",
					Name:       externalSource.Name,
					Namespace:  externalSource.Namespace,
				},
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

		// Update status after creation (status subresources cannot be set during creation)
		newArtifact.Status.Artifact = artifact
		if err := r.Status().Update(ctx, newArtifact); err != nil {
			return fmt.Errorf("failed to update ExternalArtifact status: %w", err)
		}

		return nil
	}

	// Update existing ExternalArtifact if needed
	needsUpdate := existingArtifact.Status.Artifact == nil ||
		existingArtifact.Status.Artifact.URL != artifactURL ||
		existingArtifact.Status.Artifact.Revision != revision ||
		(existingArtifact.Status.Artifact != nil && !mapsEqual(existingArtifact.Status.Artifact.Metadata, artifactMetadata))

	if needsUpdate {
		log.Info("Updating ExternalArtifact", "name", artifactName, "url", artifactURL, "revision", revision)
		existingArtifact.Status.Artifact = artifact
		if err := r.Status().Update(ctx, existingArtifact); err != nil {
			return fmt.Errorf("failed to update ExternalArtifact status: %w", err)
		}
	}

	return nil
}

// reconcileDelete handles the deletion of an ExternalSource
//
//nolint:unparam // ctrl.Result is always nil but required by interface contract for future extensibility
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
	apimeta.RemoveStatusCondition(&externalSource.Status.Conditions, ExecutingHooksCondition)
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
	// TransientError represents a temporary error that may be retried
	TransientError ErrorType = iota
	// PermanentError represents an error that should not be retried
	PermanentError
	// ConfigurationError represents an error in the resource configuration
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

	if retryCount >= r.Config.Retry.MaxAttempts {
		return 0 // No more retries
	}

	// Exponential backoff: baseDelay * 2^retryCount
	delay := time.Duration(float64(r.Config.Retry.BaseDelay) * math.Pow(2, float64(retryCount)))

	// Cap at maximum delay
	if delay > r.Config.Retry.MaxDelay {
		delay = r.Config.Retry.MaxDelay
	}

	// Add jitter to prevent thundering herd
	// Use a deterministic seed based on the resource to ensure consistent jitter
	seed := int64(0)
	for _, b := range []byte(externalSource.Namespace + "/" + externalSource.Name) {
		seed = seed*31 + int64(b)
	}
	rng := rand.New(rand.NewSource(seed + int64(retryCount)))

	jitter := time.Duration(float64(delay) * r.Config.Retry.JitterFactor * (2*rng.Float64() - 1))
	delay += jitter

	// Ensure minimum delay
	if delay < r.Config.Retry.BaseDelay {
		delay = r.Config.Retry.BaseDelay
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

// executeHooks executes a list of hooks on the input data
func (r *ExternalSourceReconciler) executeHooks(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource, input []byte, hookSpecs []sourcev1alpha1.HookSpec) ([]byte, error) {
	log := logf.FromContext(ctx)

	data := input
	maxRetries := externalSource.Spec.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default
	}
	totalRetries := 0

	for _, hookSpec := range hookSpecs {
		hookName := hookSpec.Name
		retryPolicy := hookSpec.RetryPolicy
		if retryPolicy == "" {
			retryPolicy = "fail" // Default
		}

		log.Info("Executing hook", "name", hookName, "retryPolicy", retryPolicy)

		// Execute hook with retries based on retry policy
		var hookErr error
		var output []byte
		attempts := 0
		maxAttempts := 1

		if retryPolicy == "retry" {
			maxAttempts = maxRetries - totalRetries + 1
			if maxAttempts < 1 {
				maxAttempts = 1
			}
		}

		for attempts < maxAttempts {
			hookStartTime := time.Now()
			output, hookErr = r.HookExecutor.Execute(ctx, data, hookSpec)
			hookDuration := time.Since(hookStartTime)

			// Record hook execution metrics
			if r.MetricsRecorder != nil {
				r.MetricsRecorder.RecordHookExecution(hookName, retryPolicy, hookErr == nil, hookDuration)
			}

			if hookErr == nil {
				log.Info("Hook executed successfully", "name", hookName, "attempts", attempts+1)
				data = output
				break
			}

			attempts++
			totalRetries++

			log.Info("Hook execution failed", "name", hookName, "attempt", attempts, "error", hookErr)

			if retryPolicy == "ignore" {
				log.Info("Hook failure ignored due to retry policy", "name", hookName)
				break
			}

			if retryPolicy == "retry" && attempts < maxAttempts && totalRetries < maxRetries {
				// Add a small delay between retries
				time.Sleep(time.Second * time.Duration(attempts))
				continue
			}

			// Either "fail" policy or max retries exceeded
			if retryPolicy == "fail" || totalRetries >= maxRetries {
				return nil, fmt.Errorf("hook %s failed after %d attempts: %w", hookName, attempts, hookErr)
			}
		}
	}

	return data, nil
}

// needsRecovery determines if the controller needs to perform recovery after restart
func (r *ExternalSourceReconciler) needsRecovery(externalSource *sourcev1alpha1.ExternalSource) bool {
	// Check if there are any in-progress conditions that suggest the controller was interrupted
	inProgressConditions := []string{FetchingCondition, ExecutingHooksCondition, StoringCondition}

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
	// Initialize components if not already set
	if r.GeneratorFactory == nil {
		r.GeneratorFactory = generator.NewFactory()
	}

	if r.HookExecutor == nil {
		// Load whitelist manager
		whitelistManager, err := hooks.NewFileWhitelistManager(r.Config.Hooks.WhitelistPath)
		if err != nil {
			return fmt.Errorf("failed to load hook whitelist: %w", err)
		}

		// Create hook executor
		r.HookExecutor = hooks.NewSidecarExecutor(
			r.Config.Hooks.SidecarEndpoint,
			whitelistManager,
			r.Config.Hooks.DefaultTimeout,
		)
	}

	if r.ArtifactManager == nil {
		var storageBackend storage.StorageBackend

		// Use externally provided storage backend if available (for sharing with artifact server)
		if r.StorageBackend != nil {
			storageBackend = r.StorageBackend
		} else {
			// Create storage backend based on configuration
			switch r.Config.Storage.Backend {
			case "s3":
				storageBackend = storage.NewS3Backend(storage.S3Config{
					Endpoint:  r.Config.Storage.S3.Endpoint,
					Bucket:    r.Config.Storage.S3.Bucket,
					Region:    r.Config.Storage.S3.Region,
					AccessKey: r.Config.Storage.S3.AccessKeyID,
					SecretKey: r.Config.Storage.S3.SecretAccessKey,
					UseSSL:    r.Config.Storage.S3.UseSSL,
				})
			case "memory":
				// Build base URL for memory backend if artifact server is enabled
				var baseURL string
				if r.Config.ArtifactServer.Enabled {
					baseURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
						r.Config.ArtifactServer.ServiceName,
						r.Config.ArtifactServer.ServiceNamespace,
						r.Config.ArtifactServer.Port)
				}
				storageBackend = storage.NewMemoryBackend(baseURL)
			case "pvc":
				// Build base URL for PVC backend if artifact server is enabled
				var baseURL string
				if r.Config.ArtifactServer.Enabled {
					baseURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
						r.Config.ArtifactServer.ServiceName,
						r.Config.ArtifactServer.ServiceNamespace,
						r.Config.ArtifactServer.Port)
				}
				var err error
				storageBackend, err = storage.NewPVCBackend(r.Config.Storage.PVC.Path, baseURL)
				if err != nil {
					return fmt.Errorf("failed to create PVC storage backend: %w", err)
				}
			default:
				return fmt.Errorf("unsupported storage backend: %s", r.Config.Storage.Backend)
			}

			r.StorageBackend = storageBackend
		}

		r.ArtifactManager = artifact.NewManager(storageBackend)
	}

	// Register built-in generators with HTTP client configuration
	if err := r.GeneratorFactory.RegisterGenerator("http", func() generator.SourceGenerator {
		return generator.NewHTTPGeneratorWithConfig(r.Client, &generator.HTTPClientConfig{
			Timeout:             r.Config.HTTP.Timeout,
			MaxIdleConns:        r.Config.HTTP.MaxIdleConns,
			MaxIdleConnsPerHost: r.Config.HTTP.MaxIdleConnsPerHost,
			MaxConnsPerHost:     r.Config.HTTP.MaxConnsPerHost,
			IdleConnTimeout:     r.Config.HTTP.IdleConnTimeout,
			UserAgent:           r.Config.HTTP.UserAgent,
		})
	}); err != nil {
		return fmt.Errorf("failed to register HTTP generator: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sourcev1alpha1.ExternalSource{}).
		Owns(&sourcev1.ExternalArtifact{}).
		Named("externalsource").
		Complete(r)
}
