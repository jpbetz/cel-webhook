package main

import (
	"encoding/json"
	"os"

	"k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func wasmRunner(functionName string) admitv1Func {
	return func(ar v1.AdmissionReview) *v1.AdmissionResponse {
		klog.V(2).Info("calling run WebAssembly webhook")
		obj := struct {
			metav1.ObjectMeta `json:"metadata,omitempty"`
		}{}
		raw := ar.Request.Object.Raw
		err := json.Unmarshal(raw, &obj)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}

		b, err := os.ReadFile("simple.wasm")
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		if err := execWasm(b, functionName); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		reviewResponse := v1.AdmissionResponse{}
		reviewResponse.Allowed = true
		return &reviewResponse
	}
}
