TODO:

- [ ] Find Defaulting cases to support
- [ ] Find conversion cases to support (field rename is an obvious one)
- [ ] Add 1st class OpenAPI type support somehow, like exists for protobuf
- [ ] Add support for returning validation failure reason
- [ ] support CRD deletion
- [ ] don't traverse entire object for each validation, instead, use paths to dereference into an object and run compiled validators
- [ ] try out more validator cases for builtin types (namespace selector, ...)
- [ ] support multiple validation rules on any data element
- [ ] Write unit test suite
- [ ] should there be restrictions on where valuation rules can be set?


Validation cases to support:

- ValidateCondition - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go#L222
- ValidateLabelSelectorRequirement - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go#L43
- IsQualifiedName - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L42
- IsFullyQualifiedName - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L78
- IsValidPortNum - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L277
- IsValidPortName - good error messages important! - 


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
