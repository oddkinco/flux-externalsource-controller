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
	"fmt"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sourcev1alpha1 "github.com/example/externalsource-controller/api/v1alpha1"
)

// ExternalSourceReconciler reconciles a ExternalSource object
type ExternalSourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	// ExternalSourceFinalizer is the finalizer used by the ExternalSource controller
	ExternalSourceFinalizer = "source.example.com/externalsource-finalizer"
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

	// TODO: Implement actual reconciliation logic in future tasks
	// For now, just set a progressing condition
	r.setCondition(&externalSource, ReadyCondition, metav1.ConditionUnknown, ProgressingReason, "Reconciliation logic not yet implemented")

	// Update status
	if err := r.Status().Update(ctx, &externalSource); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed", "requeue_after", interval)
	return ctrl.Result{RequeueAfter: interval}, nil
}

// reconcileDelete handles the deletion of an ExternalSource
func (r *ExternalSourceReconciler) reconcileDelete(ctx context.Context, externalSource *sourcev1alpha1.ExternalSource) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// TODO: Implement cleanup logic in future tasks
	// - Clean up artifacts from storage
	// - Clean up child ExternalArtifact resources

	log.Info("Cleaning up ExternalSource")

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

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sourcev1alpha1.ExternalSource{}).
		Named("externalsource").
		Complete(r)
}
