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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/mmohamed/managed-namespace/api/v1alpha1"
)

var _ = Describe("ManagedNamespaceConfiguration Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const invalidResourceName = "invalid-test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}

		typeNamespacedNameInvalid := types.NamespacedName{
			Name: invalidResourceName,
		}

		managednamespaceconfiguration := &operatorv1alpha1.ManagedNamespaceConfiguration{}
		invalidmanagednamespaceconfiguration := &operatorv1alpha1.ManagedNamespaceConfiguration{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ManagedNamespaceConfiguration")
			err := k8sClient.Get(ctx, typeNamespacedName, managednamespaceconfiguration)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.ManagedNamespaceConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedNameInvalid, invalidmanagednamespaceconfiguration)
			if err != nil && errors.IsNotFound(err) {
				invalidResource := &operatorv1alpha1.ManagedNamespaceConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: invalidResourceName,
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
								Content: string("a=b"),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, invalidResource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ManagedNamespaceConfiguration")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			invalidResource := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err = k8sClient.Get(ctx, typeNamespacedNameInvalid, invalidResource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ManagedNamespaceConfiguration")
			Expect(k8sClient.Delete(ctx, invalidResource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ManagedNamespaceConfigurationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resourceConfig := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err = k8sClient.Get(ctx, typeNamespacedName, resourceConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(resourceConfig.Status.Conditions).To(HaveLen(2))
			var conditions []metav1.Condition
			Expect(resourceConfig.Status.Conditions).To(ContainElement(
				HaveField("Type", Equal("Available")), &conditions))
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(conditions[0].Reason).To(Equal("Available"))

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameInvalid,
			})
			// invalid resource put error statut without returning error
			// to avoid reconcile with same configuration
			Expect(err).NotTo(HaveOccurred())

			invalidResourceConfig := &operatorv1alpha1.ManagedNamespaceConfiguration{}
			err = k8sClient.Get(ctx, typeNamespacedNameInvalid, invalidResourceConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(invalidResourceConfig.Status.Conditions).To(HaveLen(2))
			var invalidConditions []metav1.Condition
			Expect(invalidResourceConfig.Status.Conditions).To(ContainElement(
				HaveField("Type", Equal("Degraded")), &invalidConditions))
			Expect(invalidConditions).To(HaveLen(1))
			Expect(invalidConditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(invalidConditions[0].Reason).To(Equal("ReconciliationError"))
		})
	})
})
