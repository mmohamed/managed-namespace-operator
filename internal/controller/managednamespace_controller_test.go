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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.yaml.in/yaml/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/mmohamed/managed-namespace/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ManagedNamespace Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}
		managednamespaceconfiguration := &operatorv1alpha1.ManagedNamespaceConfiguration{}
		managednamespace := &operatorv1alpha1.ManagedNamespace{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ManagedNamespace and ManagedNamespaceConfiguration")

			err := k8sClient.Get(ctx, typeNamespacedName, managednamespaceconfiguration)
			if err != nil && errors.IsNotFound(err) {

				resourceRoleBindingContent, _ := yaml.Marshal(map[string]any{
					"roleRef": map[string]any{
						"kind":     "ClusterRole",
						"name":     "cluster-admin",
						"apiGroup": "rbac.authorization.k8s.io",
					},
					"subjects": []map[string]any{
						{
							"kind": "Group",
							"name": "project-group",
						},
					},
				})

				resourceConfigMapContent, _ := yaml.Marshal(map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"annotations/description": "annotation-content",
						},
						"labels": map[string]any{
							"labels/description": "labels-content",
						},
					},
					"data": map[string]any{
						"dbname": "dbname-__TARGET__",
						"path":   "/",
					},
				})

				resourceClusterRoleContent, _ := yaml.Marshal(map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"labels/description": "labels-content",
						},
					},
					"rules": []map[string]any{
						{
							"apiGroups": []string{""},
							"resources": []string{"nodes"},
							"verbs":     []string{"get"},
						},
					},
				})
				// load cacert for testing from local file
				pwd, err := os.Getwd()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				cacert, errRead := os.ReadFile(filepath.Join(pwd, "./../../test/test.cacert"))
				if errRead != nil {
					fmt.Println(errRead)
					os.Exit(1)
				}

				resourceConfig := &operatorv1alpha1.ManagedNamespaceConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
					Spec: operatorv1alpha1.ManagedNamespaceConfigurationSpec{
						Suspended: false,
						Resources: []operatorv1alpha1.Resources{
							{
								Resource: operatorv1alpha1.Resource{
									Kind:       "RoleBinding",
									ApiVersion: "rbac.authorization.k8s.io/v1",
									Name:       "admin-rolebiding",
								},
								Content: string(resourceRoleBindingContent),
							},
							{
								Resource: operatorv1alpha1.Resource{
									Kind:       "ConfigMap",
									ApiVersion: "v1",
									Name:       "mn-configmap",
									Namespace:  "default",
								},
								Content: string(resourceConfigMapContent),
							},
							{
								Resource: operatorv1alpha1.Resource{
									Kind:       "ClusterRole",
									ApiVersion: "rbac.authorization.k8s.io/v1",
									Name:       "mn-node-cluster-role",
								},
								Content: string(resourceClusterRoleContent),
							},
						},
						Callbacks: []operatorv1alpha1.Callbacks{
							{
								URI:          "https://www.google.com?target=__TARGET__",
								Method:       "GET",
								SuccessCodes: []int{200, 201},
								Headers: []operatorv1alpha1.HTTPHeader{
									{
										Name:  "CUSTOM_HTTP_HEADER",
										Value: "custom-http-header-value",
									},
									{
										Name:  "CUSTOM_HTTP_HEADER_WITH_TARGET",
										Value: "custom-http-header-value-__TARGET__",
									},
								},
								CACert: string(cacert),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resourceConfig)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedName, managednamespace)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.ManagedNamespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resourceConfig := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err := k8sClient.Get(ctx, typeNamespacedName, resourceConfig)
			Expect(err).NotTo(HaveOccurred())

			resource := &operatorv1alpha1.ManagedNamespace{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ManagedNamespace and ManagedNamespaceConfiguration")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ManagedNamespaceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// check namespace creation
			namespace := &corev1.Namespace{}
			errNS := k8sClient.Get(ctx, typeNamespacedName, namespace)
			Expect(errNS).NotTo(HaveOccurred())

			refferedTo, found := namespace.Annotations[referredAnnotation]
			Expect(found).To(BeTrue())
			Expect(refferedTo).To(Equal(resourceName))

			rolebinding := &rbac.RoleBinding{}
			errRB := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "admin-rolebiding-" + resourceName,
				Namespace: resourceName,
			}, rolebinding)
			Expect(errRB).NotTo(HaveOccurred())

			managedNamespaceAnnotationContent, found := rolebinding.Annotations[managedNamespaceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceAnnotationContent).To(Equal(resourceName))

			managedNamespaceConfigurationAnnotationContent, found := rolebinding.Annotations[managedNamespaceConfigurationAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceConfigurationAnnotationContent).To(Equal(resourceName))

			managedNamespaceResourceAnnotationContent, found := rolebinding.Annotations[managedNamespaceResourceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceResourceAnnotationContent).To(Equal("rbac.authorization.k8s.io/v1/RoleBinding/admin-rolebiding"))

			Expect(rolebinding.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(rolebinding.RoleRef.Name).To(Equal("cluster-admin"))
			Expect(rolebinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))

			// check owner ref
			Expect(rolebinding.GetOwnerReferences()).To(HaveLen(1))
			Expect(rolebinding.GetOwnerReferences()[0].Kind).To(Equal("ManagedNamespace"))
			Expect(rolebinding.GetOwnerReferences()[0].Name).To(Equal(resourceName))

			// check configmap in default namespace
			configmap := &corev1.ConfigMap{}
			errCM := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "mn-configmap-" + resourceName,
				Namespace: "default",
			}, configmap)
			Expect(errCM).NotTo(HaveOccurred())

			managedNamespaceAnnotationContent, found = configmap.Annotations[managedNamespaceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceAnnotationContent).To(Equal(resourceName))

			managedNamespaceConfigurationAnnotationContent, found = configmap.Annotations[managedNamespaceConfigurationAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceConfigurationAnnotationContent).To(Equal(resourceName))

			managedNamespaceResourceAnnotationContent, found = configmap.Annotations[managedNamespaceResourceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceResourceAnnotationContent).To(Equal("v1/ConfigMap/mn-configmap"))

			// check owner ref
			Expect(configmap.GetOwnerReferences()).To(HaveLen(1))
			Expect(configmap.GetOwnerReferences()[0].Kind).To(Equal("ManagedNamespace"))
			Expect(configmap.GetOwnerReferences()[0].Name).To(Equal(resourceName))

			// check data
			Expect(configmap.Data["dbname"]).To(Equal("dbname-" + resourceName))

			// check clusterrole
			clusterrole := &rbac.ClusterRole{}
			errCR := k8sClient.Get(ctx, types.NamespacedName{
				Name: "mn-node-cluster-role-" + resourceName,
			}, clusterrole)
			Expect(errCR).NotTo(HaveOccurred())

			managedNamespaceAnnotationContent, found = clusterrole.Annotations[managedNamespaceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceAnnotationContent).To(Equal(resourceName))

			managedNamespaceConfigurationAnnotationContent, found = clusterrole.Annotations[managedNamespaceConfigurationAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceConfigurationAnnotationContent).To(Equal(resourceName))

			managedNamespaceResourceAnnotationContent, found = clusterrole.Annotations[managedNamespaceResourceAnnotation]
			Expect(found).To(BeTrue())
			Expect(managedNamespaceResourceAnnotationContent).To(Equal("rbac.authorization.k8s.io/v1/ClusterRole/mn-node-cluster-role"))

			// check status
			resource := &operatorv1alpha1.ManagedNamespace{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Status.Conditions).To(HaveLen(2))
			var conditions []metav1.Condition
			Expect(resource.Status.Conditions).To(ContainElement(
				HaveField("Type", Equal("Available")), &conditions))
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(conditions[0].Reason).To(Equal("Available"))

			resourceConfig := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err = k8sClient.Get(ctx, typeNamespacedName, resourceConfig)
			Expect(err).NotTo(HaveOccurred())
			// only managed namespace are reconciled
			Expect(resourceConfig.Status.Conditions).To(BeEmpty())

			// Update config
			resourceConfigMapContent, _ := yaml.Marshal(map[string]any{
				"data": map[string]any{
					"dbname": "dbname",
					"path":   "/",
				},
			})
			resourceConfig.Spec.Resources[1].Content = string(resourceConfigMapContent)

			Expect(k8sClient.Update(ctx, resourceConfig)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			errCM = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "mn-configmap-" + resourceName,
				Namespace: "default",
			}, configmap)
			Expect(errCM).NotTo(HaveOccurred())

			// check data
			Expect(configmap.Data["dbname"]).To(Equal("dbname"))
		})
	})
})
