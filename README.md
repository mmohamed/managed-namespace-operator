# Managed Namespace Kubernetes Operator

## Description
A Kubernetes operator can be designed to enable cluster administrators to delegate namespace-level management to individual users while maintaining strict isolation across the cluster. This operator automates the creation and configuration of dedicated namespaces for each user, along with the appropriate Role-Based Access Control (RBAC) policies.

With this approach, users are granted full administrative control over their own namespace, allowing them to deploy, manage, and scale applications independently. At the same time, access to other namespaces in the cluster is strictly restricted, ensuring security and multi-tenancy.

The operator continuously enforces these access rules and can automatically reconcile any configuration drift, ensuring that permissions remain compliant with the intended policies. This provides a scalable and secure way to empower users without compromising the integrity of the overall cluster.

## Concept and configuration

### How it's work
When a cluster administrator grants a user permission to manage `ManagedNamespace` resources, a `Namespace` with the same name will be created for each `ManagedNamespace` created by the user. Additionally, the controller will create all resources defined by the administrator through `ManagedNamespaceConfiguration` resources and will execute all callbacks during every reconciliation process.
In case of, the new created resources need to be linked to the target namespace, a slug `__TARGET__` can be used in resources content and callback URI / HTTP Header value, the it will to be replaced with target `ManagedNamespace`

```yaml
kind: ManagedNamespace
apiVersion: operator.medinvention.io/v1alpha1
metadata:
    name: myproject
```

### Configuration

## Resources
Resources example : A new RoleBinding will be created within each new `ManagedNamespace`, and a copy of the ConfigMap (mn-configmap-xxx) will be created in the default namespace for each `ManagedNamespace`. This also works with cluster-wide resources.

```yaml
kind: ManagedNamespaceConfiguration
apiVersion: operator.medinvention.io/v1alpha1
metadata:
    name: main-config
spec:
    resources:
        - resource: 
            kind: RoleBinding
            apiVersion: rbac.authorization.k8s.io/v1
            name: admin-rolebiding
          content: |
            roleRef:
                kind: ClusterRole
                name: cluster-admin
                apiGroup: rbac.authorization.k8s.io
            subjects:
                - kind: Group
                  name: project-group
        - resource: 
            kind: ConfigMap
            apiVersion: v1
            name: mn-configmap
            namespace: default
          content: |
            metadata:
                annotations:
                    annotations/description: annotation-content
                labels:
                    labels/description: label-content
            data:
                dbname: dbname-__TARGET__
                path : "/"
```

## Callbacks
Callbacks example : For each reconciliation process of a `ManagedNamespace`, a callback will be executed.

```yaml
kind: ManagedNamespaceConfiguration
apiVersion: operator.medinvention.io/v1alpha1
metadata:
    name: main-config
spec:
    callbacks:
        - uri: "https://www.google.com?target=__TARGET__"
          method: "GET"
          successcodes: [200, 201]
          headers:
            - name: CUSTOM_HTTP_HEADER
              value: custom-http-header-value
            - name: CUSTOM_HTTP_HEADER_WITH_TARGET
              value: custom-http-header-value-__TARGET__
          cacert: | 
            -----BEGIN CERTIFICATE-----
            xxxxxxxxxxxxxxxxxxxxxx
```

## Usage helm chart

```sh
helm upgrade --install managed-namespace dist/chart \
    --namespace managed-namespace-system \
    --create-namespace \
    --set manager.image.repository=medinvention/managed-namespace \
    --set manager.image.tag=dev \
    --wait \
    --timeout 5m 
```

> **NOTE**: If you encounter RBAC errors, you may need to grant the `managed-namespace-controller-manager` service account the necessary permissions to allow the controller to install all resources defined in `ManagedNamespaceConfiguration` resources.
```sh
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: managed-namespace-controller-manager-cluster-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: managed-namespace-controller-manager
  namespace: managed-namespace-system
EOF
```

## Usage from source

### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.


### Deploy on the cluster 

```sh
# To Deploy on the cluster
make docker-buildx docker-push IMG=medinvention/managed-namespace:dev
# Install the CRDs into the cluster (make manifests for generation)
make install
# Deploy 
make deploy IMG=medinvention/managed-namespace:dev
# Apply sample
kubectl apply -k config/samples/
```

### Distribution

```sh
# Build the installer YAML File
make build-installer IMG=medinvention/managed-namespace:dev
# Apply manifest
kubectl apply -f https://raw.githubusercontent.com/mmohamed/managed-namespace/dev/dist/install.yaml
# Build Helm chart
kubebuilder edit --plugins=helm/v2-alpha # chart will be generated under 'dist/chart'
```

### Cleanup

```sh
# Delete the instances (CRs) from the cluster
kubectl delete -k config/samples/
# Delete the APIs(CRDs) from the cluster
make uninstall
# UnDeploy the controller from the cluster
make undeploy
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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