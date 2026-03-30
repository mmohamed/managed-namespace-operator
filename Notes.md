
```bash
kubebuilder init --domain=medinvention.io --repo=github.com/mmohamed/managed-namespace
kubebuilder create api --group=operator --version=v1alpha1 --kind=ManagedNamespace --namespaced=false
kubebuilder create api --group=operator --version=v1alpha1 --kind=ManagedNamespaceConfiguration --namespaced=false

make manifests

make install
make run # locally
kubectl apply -f test/testdata.yaml
```


# TODO
- Code cleanup
- Cleanup resources on managed namespace configuration updated 
- Build and deploy
- Helm package