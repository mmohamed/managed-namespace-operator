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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/mmohamed/managed-namespace/api/v1alpha1"
	mergemap "github.com/mmohamed/managed-namespace/internal/utils"
)

// ManagedNamespaceReconciler reconciles a ManagedNamespace object
type ManagedNamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Definitions to manage status conditions
const (
	// typeAvailableManagedNamespace represents the status of the ManagedNamespace reconciliation
	typeAvailableManagedNamespace = "Available"
	// typeProgressingManagedNamespace represents the status used when the ManagedNamespace is being reconciled
	typeProgressingManagedNamespace = "Progressing"
	// typeDegradedManagedNamespace represents the status used when the ManagedNamespace has encountered an error
	typeDegradedManagedNamespace = "Degraded"
)

var (
	referredAnnotation                      = "managednamespace.operator.medinvention.io/referred-to"
	managedNamespaceConfigurationAnnotation = "managednamespace.operator.medinvention.io/managednamespaceconfiguration"
	managedNamespaceAnnotation              = "managednamespace.operator.medinvention.io/managednamespace"
	managedNamespaceResourceAnnotation      = "managednamespace.operator.medinvention.io/managednamespaceresource"
	targetSlug                              = "__TARGET__"
)

// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.medinvention.io,resources=managednamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces/status,verbs=get

func (r *ManagedNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Starting reconciliation")

	// Fetch the ManagedNamespace instance
	var managedNamespace operatorv1alpha1.ManagedNamespace
	if err := r.Get(ctx, req.NamespacedName, &managedNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ManagedNamespace not found, Ignoring...")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Error(err, "Failed to get ManagedNamespace")
		return ctrl.Result{}, err
	}

	// Initialize status conditions if not yet present
	if len(managedNamespace.Status.Conditions) == 0 {
		meta.SetStatusCondition(&managedNamespace.Status.Conditions, metav1.Condition{
			Type:    typeProgressingManagedNamespace,
			Status:  metav1.ConditionUnknown,
			Reason:  "Reconciling",
			Message: "Starting reconciliation",
		})
		if err := r.Status().Update(ctx, &managedNamespace); err != nil {
			log.Error(err, "Failed to update ManagedNamespace status")
			return ctrl.Result{}, err
		}
		// re-fetch
		if err := r.Get(ctx, req.NamespacedName, &managedNamespace); err != nil {
			log.Error(err, "Failed to re-fetch ManagedNamespace")
			return ctrl.Result{}, err
		}
		// last sync time
		patch := fmt.Appendf(nil, `{"status":{"lastSyncTime":"%s"}}`, time.Now().Format(time.RFC3339))
		if statusErr := r.Status().Patch(ctx, &managedNamespace, client.RawPatch(types.MergePatchType, patch)); statusErr != nil {
			log.Error(statusErr, "Failed to update ManagedNamespace status last sync time")
			return ctrl.Result{}, statusErr
		}
		// re-fetch
		if err := r.Get(ctx, req.NamespacedName, &managedNamespace); err != nil {
			log.Error(err, "Failed to re-fetch ManagedNamespace")
			return ctrl.Result{}, err
		}
	}

	var namespace corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("Namespace '%s' not found, Creating...", req.Name))

			ns := &unstructured.Unstructured{}
			ns.Object = map[string]any{
				"metadata": map[string]any{
					"name": req.Name,
					"annotations": map[string]any{
						referredAnnotation: req.Name,
					},
				},
				"spec": map[string]any{},
			}
			ns.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Namespace",
				Version: "v1",
			})

			if err := r.Create(ctx, ns); err != nil {
				log.Error(err, fmt.Sprintf("Failed to create Namespace '%s' !", req.Name))
				// Update status condition to reflect the error
				meta.SetStatusCondition(&managedNamespace.Status.Conditions, metav1.Condition{
					Type:    typeDegradedManagedNamespace,
					Status:  metav1.ConditionFalse,
					Reason:  "ReconciliationError",
					Message: fmt.Sprintf("Failed to create Namespace: %s , %v", req.Name, err),
				})
				if statusErr := r.Status().Update(ctx, &managedNamespace); statusErr != nil {
					log.Error(statusErr, "Failed to update ManagedNamespace status")
				}
				return ctrl.Result{}, err
			}
			// ref-fetch namespace
			if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, ns); err != nil {
				log.Error(err, "Failed to re-fetch namespace")
				return ctrl.Result{}, err
			}
			// Set ownerRef
			if err := ctrl.SetControllerReference(&managedNamespace, ns, r.Scheme); err != nil {
				log.Error(err, "Failed to set the OwnerRef on namespace")
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, ns); err != nil {
				log.Error(err, "Failed to save the OwnerRef on namespace")
				return ctrl.Result{}, err
			}
			// refresh
			if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &namespace); err != nil {
				log.Error(err, "Failed to re-fetch namespace")
				return ctrl.Result{}, err
			}
			if err := r.Get(ctx, req.NamespacedName, &managedNamespace); err != nil {
				log.Error(err, "Failed to re-fetch ManagedNamespace")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, fmt.Sprintf("Failed to get Namespace: %s", req.Name))
			return ctrl.Result{}, err
		}
	}
	refferedTo, ok := namespace.Annotations[referredAnnotation]
	if !ok || refferedTo != req.Name {
		log.Error(nil, fmt.Sprintf("Found Namespace %s but not managed by ManagedNamespace controller", req.Name))
		return ctrl.Result{}, nil
	}
	log.Info(fmt.Sprintf("Namespace '%s' found, updating resources...", req.Name))
	var configurations operatorv1alpha1.ManagedNamespaceConfigurationList
	if err := r.List(ctx, &configurations); err != nil {
		log.Error(err, "Unable to list ManagedNamespaceConfiguration")
		return ctrl.Result{}, err
	}
	for _, configuration := range configurations.Items {
		if err := r.ApplyConfiguration(ctx, &managedNamespace, &configuration, &namespace); err != nil {
			log.Error(err, fmt.Sprintf("Unable to apply ManagedNamespaceConfiguration %s", configuration.Name))
			// Update status condition to reflect the error
			meta.SetStatusCondition(&managedNamespace.Status.Conditions, metav1.Condition{
				Type:    typeDegradedManagedNamespace,
				Status:  metav1.ConditionFalse,
				Reason:  "ReconciliationError",
				Message: fmt.Sprintf("Unable to apply ManagedNamespaceConfiguration %s, check logs", configuration.Name),
			})
			if statusErr := r.Status().Update(ctx, &managedNamespace); statusErr != nil {
				log.Error(statusErr, "Failed to update ManagedNamespace status")
			}
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("Configuration '%s' applied to namespace '%s'", configuration.Name, req.Name))
	}

	meta.SetStatusCondition(&managedNamespace.Status.Conditions, metav1.Condition{
		Type:    typeAvailableManagedNamespace,
		Status:  metav1.ConditionTrue,
		Reason:  "Available",
		Message: "Reconciling done with success",
	})
	if err := r.Status().Update(ctx, &managedNamespace); err != nil {
		log.Error(err, "Failed to update ManagedNamespace status")
		return ctrl.Result{}, err
	}
	// re-fetch
	if err := r.Get(ctx, req.NamespacedName, &managedNamespace); err != nil {
		log.Error(err, "Failed to re-fetch ManagedNamespace")
		return ctrl.Result{}, err
	}
	// last sync time
	patch := fmt.Appendf(nil, `{"status":{"lastSyncTime":"%s"}}`, time.Now().Format(time.RFC3339))
	if statusErr := r.Status().Patch(ctx, &managedNamespace, client.RawPatch(types.MergePatchType, patch)); statusErr != nil {
		log.Error(statusErr, "Failed to update ManagedNamespace status last sync time")
		return ctrl.Result{}, statusErr
	}

	log.Info(fmt.Sprintf("All configuration are applied to namespace '%s'", req.Name))

	return ctrl.Result{}, nil
}

func (r *ManagedNamespaceReconciler) ApplyConfiguration(ctx context.Context, managedNamespace *operatorv1alpha1.ManagedNamespace, configuration *operatorv1alpha1.ManagedNamespaceConfiguration, namespace *corev1.Namespace) error {
	log := logf.FromContext(ctx)
	// not ressource to apply
	if configuration.Spec.Suspended {
		return nil
	}
	if len(configuration.Spec.Resources) == 0 {
		return nil
	}
	for _, resource := range configuration.Spec.Resources {
		type Data map[string]any
		out := Data{}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(resource.Content, targetSlug, namespace.Name)), &out); err != nil {
			log.Error(err, fmt.Sprintf("Unable to decode YAML content of resource %s of configuration %s", resource.Resource.Name, configuration.Name))
			return err
		}
		rs := &unstructured.Unstructured{}
		rs.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    resource.Resource.Kind,
			Version: resource.Resource.ApiVersion,
		})

		rsName := fmt.Sprintf("%s-%s", resource.Resource.Name, namespace.Name)
		rsSelector := client.ObjectKey{Namespace: namespace.Name, Name: rsName}
		if len(resource.Resource.Namespace) > 0 {
			rsSelector = client.ObjectKey{Namespace: resource.Resource.Namespace, Name: rsName}
		} else if resource.Resource.ClusterResource {
			rsSelector = client.ObjectKey{Name: rsName}
		}
		newOne := false
		// check if exist
		if err := r.Get(ctx, rsSelector, rs); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("Unable to get resource %s of configuration %s", resource.Resource.Name, configuration.Name))
				return err
			} else {
				newOne = true
			}
		}
		// found, check management
		if !newOne {
			annotations := rs.Object["metadata"].(map[string]any)["annotations"]
			unmanaged := false

			managedNamespaceConfigurationAnnotationContent, ok := annotations.(map[string]any)[managedNamespaceConfigurationAnnotation]
			unmanaged = !ok || managedNamespaceConfigurationAnnotationContent != configuration.Name
			managedNamespaceAnnotationContent, ok := annotations.(map[string]any)[managedNamespaceAnnotation]
			unmanaged = unmanaged || !ok || managedNamespaceAnnotationContent != namespace.Name
			managedNamespaceResourceAnnotationContent, ok := annotations.(map[string]any)[managedNamespaceResourceAnnotation]
			unmanaged = unmanaged || !ok || managedNamespaceResourceAnnotationContent != fmt.Sprintf("%s/%s/%s", resource.Resource.ApiVersion, resource.Resource.Kind, resource.Resource.Name)

			if unmanaged {
				log.Error(nil, fmt.Sprintf("Unmanaged resource %s of configuration %s already found !", resource.Resource.Name, configuration.Name))
				return fmt.Errorf("unmanaged resource %s of configuration %s already found", resource.Resource.Name, configuration.Name)
			}
		}

		rs.Object = map[string]any{
			"metadata": map[string]any{
				"name": rsName,
				"annotations": map[string]any{
					managedNamespaceConfigurationAnnotation: configuration.Name,
					managedNamespaceAnnotation:              namespace.Name,
					managedNamespaceResourceAnnotation:      fmt.Sprintf("%s/%s/%s", resource.Resource.ApiVersion, resource.Resource.Kind, resource.Resource.Name),
				},
			},
		}

		rs.Object = mergemap.Merge(rs.Object, out)

		// set namespace
		if len(resource.Resource.Namespace) > 0 {
			rs.Object["metadata"].(map[string]any)["namespace"] = resource.Resource.Namespace
		} else if !resource.Resource.ClusterResource {
			rs.Object["metadata"].(map[string]any)["namespace"] = namespace.Name
		}

		// Set ownerRef
		if err := ctrl.SetControllerReference(managedNamespace, rs, r.Scheme); err != nil {
			log.Error(err, fmt.Sprintf("Failed to set the OwnerRef on resource %s of configuration %s", resource.Resource.Name, configuration.Name))
			return err
		}

		rs.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    resource.Resource.Kind,
			Version: resource.Resource.ApiVersion,
		})

		if newOne {
			if err := r.Create(ctx, rs); err != nil {
				log.Error(err, fmt.Sprintf("Unable to create resource %s of configuration %s", resource.Resource.Name, configuration.Name))
				return err
			}
		} else {
			if err := r.Update(ctx, rs); err != nil {
				log.Error(err, fmt.Sprintf("Unable to update resource %s of configuration %s", resource.Resource.Name, configuration.Name))
				return err
			}
		}
	}
	// callbacks
	for _, callback := range configuration.Spec.Callbacks {
		if err := ExecuteCallback(ctx, &callback, configuration, namespace); err != nil {
			return err
		}
	}
	return nil
}

func ExecuteCallback(ctx context.Context, callback *operatorv1alpha1.Callbacks, configuration *operatorv1alpha1.ManagedNamespaceConfiguration, namespace *corev1.Namespace) error {
	log := logf.FromContext(ctx)
	// init http client
	callbaclClient := &http.Client{}
	// ssl config
	if len(callback.CACert) > 0 {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(callback.CACert))
		callbaclClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
		}
	}
	// build request
	method := "GET"
	if len(callback.Method) > 0 {
		method = callback.Method
	}
	uri := strings.ReplaceAll(callback.URI, targetSlug, namespace.Name)
	request, err := http.NewRequest(method, uri, nil)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed creating callback request of %s of configuration %s", uri, configuration.Name))
		return err
	}
	// http headers
	for _, header := range callback.Headers {
		request.Header.Add(header.Name, strings.ReplaceAll(header.Value, targetSlug, namespace.Name))
	}
	// execute
	response, err := callbaclClient.Do(request)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed callback of %s of configuration %s", uri, configuration.Name))
		return err
	}
	// check status code
	if (len(callback.SuccessCodes) == 0 && (response.StatusCode < 200 || response.StatusCode > 299)) || (len(callback.SuccessCodes) > 0 && !slices.Contains(callback.SuccessCodes, response.StatusCode)) {
		statusCodeError := fmt.Errorf("error response code %d of callback of %s of configuration %s", response.StatusCode, uri, configuration.Name)
		log.Error(statusCodeError, fmt.Sprintf("Error response code %d of callback of %s of configuration %s", response.StatusCode, uri, configuration.Name))
		return statusCodeError
	}
	log.Info(fmt.Sprintf("Success callback of %s of configuration %s with response code %d", uri, configuration.Name, response.StatusCode))
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ManagedNamespace{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("managednamespace").
		Owns(&corev1.Namespace{}).
		Complete(r)
}
