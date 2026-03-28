/*
Copyright 2026.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/mmohamed/managed-namespace/api/v1alpha1"
)

// ManagedNamespaceConfigurationReconciler reconciles a ManagedNamespaceConfiguration object
type ManagedNamespaceConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Definitions to manage status conditions
const (
	// typeAvailableManagedNamespaceConfiguration represents the status of the ManagedNamespaceConfiguration reconciliation
	typeAvailableManagedNamespaceConfiguration = "Available"
	// typeProgressingManagedNamespaceConfiguration represents the status used when the ManagedNamespaceConfiguration is being reconciled
	typeProgressingManagedNamespaceConfiguration = "Progressing"
	// typeDegradedManagedNamespaceConfiguration represents the status used when the ManagedNamespaceConfiguration has encountered an error
	typeDegradedManagedNamespaceConfiguration = "Degraded"
)

// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaceconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaceconfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaceconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ManagedNamespaceConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *ManagedNamespaceConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation")

	// Fetch the ManagedNamespaceConfiguration instance
	var ManagedNamespaceConfiguration operatorv1alpha1.ManagedNamespaceConfiguration
	if err := r.Get(ctx, req.NamespacedName, &ManagedNamespaceConfiguration); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "ManagedNamespaceConfiguration not found, Ignoring...")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Error(err, "Failed to get ManagedNamespaceConfiguration")
		return ctrl.Result{}, err
	}

	// Initialize status conditions if not yet present
	if len(ManagedNamespaceConfiguration.Status.Conditions) == 0 {
		meta.SetStatusCondition(&ManagedNamespaceConfiguration.Status.Conditions, metav1.Condition{
			Type:    typeProgressingManagedNamespaceConfiguration,
			Status:  metav1.ConditionUnknown,
			Reason:  "Reconciling",
			Message: "Starting reconciliation",
		})
		if err := r.Status().Update(ctx, &ManagedNamespaceConfiguration); err != nil {
			log.Error(err, "Failed to update ManagedNamespaceConfiguration status")
			return ctrl.Result{}, err
		}
		// re-fetch
		if err := r.Get(ctx, req.NamespacedName, &ManagedNamespaceConfiguration); err != nil {
			log.Error(err, "Failed to re-fetch ManagedNamespaceConfiguration")
			return ctrl.Result{}, err
		}
		// last sync time
		patch := []byte(fmt.Sprintf(`{"status":{"lastSyncTime":"%s"}}`, time.Now().Format(time.RFC3339)))
		if statusErr := r.Status().Patch(ctx, &ManagedNamespaceConfiguration, client.RawPatch(types.MergePatchType, patch)); statusErr != nil {
			log.Error(statusErr, "Failed to update ManagedNamespaceConfiguration status last sync time")
			return ctrl.Result{}, statusErr
		}
		// re-fetch
		if err := r.Get(ctx, req.NamespacedName, &ManagedNamespaceConfiguration); err != nil {
			log.Error(err, "Failed to re-fetch ManagedNamespaceConfiguration")
			return ctrl.Result{}, err
		}
	}
	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedNamespaceConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ManagedNamespaceConfiguration{}).
		Named("managednamespaceconfiguration").
		Complete(r)
}
