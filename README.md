TODO:

- [ ] add informer to fetch crds and register them
- [ ] add details on how to register with apiserver
- [ ] add end-to-end examples

How to run:

go build . && ./runner-webhook webhook \
  --tls-cert-file $HOME/projects/k8s/src/k8s.io/kubernetes/hack/testdata/tls.crt \
  --tls-private-key-file $HOME/projects/k8s/src/k8s.io/kubernetes/hack/testdata/tls.key \
  --port 8084 \
  -v 4

How to test directly:

curl -H "Content-Type: application/json" -kv https://localhost:8084/validate --data @example/crontab/admissionreview.json | jq .
