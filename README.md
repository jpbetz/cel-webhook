TODO:

- [ ] compile and cache all CEL programs when CRD is created/updated

How to run just the webhook:

example/crontab/wasm/setup-webhook.sh

go build . && ./runner-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4

How to test directly:

curl -H "Content-Type: application/json" -kv https://localhost:8084/validate --data @example/crontab/admissionreview.json | jq .

How to run with Kubernetes:

example/crontab/wasm/setup-webhook.sh

$ cd kubernetes
hack/local-up-cluster.sh

$ cd runner-webhook
export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
go build . && ./runner-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4
  
$ cd runner-webhook
kubectl apply --server-side -f example/crontab/crd.yaml
kubectl apply --server-side -f example/crontab/webhook.yaml
