
```bash
kubebuilder init --domain=medinvention.io --repo=github.com/mmohamed/managed-namespace
kubebuilder create api --group=operator --version=v1alpha1 --kind=ManagedNamespace --namespaced=false
kubebuilder create api --group=operator --version=v1alpha1 --kind=ManagedNamespaceConfiguration --namespaced=false

make manifests

make install
make run # locally

cat <<EOF | kubectl apply -f -
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

EOF

cat <<EOF | kubectl apply -f -
kind: ManagedNamespace
apiVersion: operator.medinvention.io/v1alpha1
metadata:
    name: project
spec: {}
EOF
```
