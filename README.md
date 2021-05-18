cat go build ./... && ./runner-webhook webhook --tls-cert-file $HOME/projects/k8s/src/k8s.io/kubernetes/hack/testdata/tls.crt --tls-private-key-file $HOME/projects/k8s/src/k8s.io/kubernetes/hack/testdata/tls.key --port 8084
curl -H "Content-Type: application/json" -kv https://localhost:8084/add-label --data @$HOME/projects/wasm/runner-webhook/admissionreview.json | jq .
