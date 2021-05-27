Omni-webhook
------------

This webhook was written to explore using embedded/sandboxed languages for the validation, defaulting 
and conversion of custom resources, with the goal of finding embedded/sandboxed languages that could
be incorporated directly into Kubernetes.

CRD schemas can be augmented to include embedded code, which this webhook acts on automatically. For
example, to add a cross field validation rule:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
...
schema:
    openAPIV3Schema:
      type: object
      properties:
        spec:
          format: "validate: minReplicas <= replicas && replicas <= maxReplicas"
          type: object
          properties:
            replicas:
              type: integer
            minReplicas:
              type: integer
            maxReplicas:
              type: integer
```

The webhook monitors CRDs for any validation, defaulting and conversion rules and then performs
them on all custom resources without the need to ever restart the webhook.

The implementations currently being explored are:

- CEL expression language
  - designed to embedded, safe, performant
  - sufficiently expressive for the vast majority of validation/defaulting/conversion needs
  - CEL expressions can be typechecked
- WebAssembly
  - versatile
  - excellent sandbox properties
  - large developer community

 TODO: explore other expression and transform languages (jq or similar?)

Usage
-----

```sh
# Set up some certs
$ example/crontab/wasm/setup-webhook.sh

# Run a kubernetes cluster, e.g.:
$ cd kubernetes
$ hack/local-up-cluster.sh

# run this webhook
$ export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
$ go build . && ./omni-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4
  
# register this webhook
$ kubectl apply --server-side -f example/crontab/webhook.yaml

# Test it out with a sample CRD
$ kubectl apply --server-side -f example/crontab/crd.yaml

# Validate a CR
$ kubectl apply --server-side -f example/crontab/invalid.yaml

# Get the CR at v2 to force conversion (v2 is the default, so this isn't strictly needed, but it's good to know how to do it)
$ kubectl get crontabs.v2.stable.example.com my-crontab -oyaml
```

Notes
-----

How to run just the webhook:

example/crontab/wasm/setup-webhook.sh

go build . && ./omni-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4

How to test directly:

curl -H "Content-Type: application/json" -kv https://localhost:8084/validate --data @example/crontab/admissionreview.json | jq .


TODO:

- [ ] Put conversion rules at root and require full paths (prefix with v1? make versions clear)
- [ ] Expand on validation cases to support
- [ ] Find Defaulting cases to support
- [ ] Add 1st class OpenAPI type support somehow, like exists for protobuf
- [ ] Add support for returning validation failure reason
- [ ] support CRD deletion
- [ ] don't traverse entire object for each validation, instead, use paths to dereference into an object and run compiled validators
- [ ] try out more validator cases for builtin types (namespace selector, ...)
- [ ] support multiple validation rules on any data element
- [ ] Write unit test suite
- [ ] should there be restrictions on where valuation rules can be set?


Validation cases:

- ValidateCondition - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go#L222
- ValidateLabelSelectorRequirement - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/validation/validation.go#L43
- IsQualifiedName - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L42
- IsFullyQualifiedName - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L78
- IsValidPortNum - https://github.com/kubernetes/kubernetes/blob/fb3273774ad0738fadd5a18693741d6818187b65/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L277
- IsValidPortName - good error messages important! - 
- Scale Status -- Selector and TargetSelector guaranteed to always represent the same thing?

Conversion cases:

How to run just the webhook:

example/crontab/wasm/setup-webhook.sh

go build . && ./omni-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4

How to test directly:

curl -H "Content-Type: application/json" -kv https://localhost:8084/validate --data @example/crontab/admissionreview.json | jq .

How to run with Kubernetes:
- Field renaming
- Field location changes (e.g.LegacyExtender: HTTPTimeout <-> HTTPTimeout.Duration)
- Writing fields into annotations for round trip(e.g. DeprecatedRollbackTo, DeprecatedTopology)
- auto populating (e.g. Convert_apps_StatefulSetSpec_To_v1beta2_StatefulSetSpec)
- string <-> structured 
  - e.g. Convert_autoscaling_ScaleStatus_To_v1beta2_ScaleStatus, Convert_Map_string_To_string_To_v1_LabelSelector
  - ServicePort <-> Port.Name, Port.Number (Convert_v1beta1_IngressBackend_To_networking_IngressBackend)
- enum mappings (e.g. apps.ReplicaSetConditionType, core.ConditionStatus)
- non-convertable error annotations? (v1.NonConvertibleAnnotationPrefix)
- conditional logic
  - PodIPs <-> PodIP (Convert_v1_PodStatus_To_core_PodStatus)
  - Remove hostname from the topology map ONLY IF it is the same value as nodeName (Convert_v1beta1_Endpoint_To_discovery_Endpoint)
- rounding? (e.g. Convert_v1_ResourceList_To_core_ResourceList)
- value translation (e.g. * <-> allAuthenticated in Convert_v1beta1_Policy_To_abac_Policy)
- consistency enforcement (e.g. policy.StripPDBV1beta1Label)
- field selector conversion to match object field changes (e.g. AddFieldLabelConversionsForEvent)
- update referenced object info to match version (e.g. Convert_rbac_Subject_To_v1alpha1_Subject)

Defaulting cases:

$ cd kubernetes
hack/local-up-cluster.sh

$ cd omni-webhook
export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
go build . && ./omni-webhook webhook \
  --tls-cert-file example/crontab/webhook.crt \
  --tls-private-key-file example/crontab/webhook.key \
  --port 8084 \
  -v 4
  
$ cd omni-webhook
kubectl apply --server-side -f example/crontab/crd.yaml
kubectl apply --server-side -f example/crontab/webhook.yaml
