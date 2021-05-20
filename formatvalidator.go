package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	v1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"

	"github.com/jpbetz/runner-webhook/validators"
)

type formatValidators struct {
	validators map[string]validators.FormatValidator
	crdSchemas map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion
}

func newFormatValidators() *formatValidators {
	v := &formatValidators{}
	v.validators = map[string]validators.FormatValidator{}
	v.crdSchemas = map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion{}
	return v
}


func (v *formatValidators) registerCrd(filepath string) error {
	crd, err := v.loadCrd(filepath)
	if err != nil {
		return err
	}
	for _, version := range crd.Spec.Versions {
		gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}
		v.crdSchemas[gvk] = &version
	}

	return nil
}

func (v *formatValidators) registerFormat(validatorId string, formatValidator validators.FormatValidator) {
	v.validators[validatorId] = formatValidator
}

func (v *formatValidators) loadCrd(crdfilepath string) (*apiextensionsv1.CustomResourceDefinition, error) {
	b, err := os.ReadFile(crdfilepath)
	if err != nil {
		return nil, err
	}
	o := &apiextensionsv1.CustomResourceDefinition{}
	err = yaml.Unmarshal(b, o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (v *formatValidators) serveValidateRequest(w http.ResponseWriter, r *http.Request) {
	serve(w, r, v.validateRequest)
}

func (v *formatValidators) validateRequest(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.V(2).Info("calling run WebAssembly webhook")
	obj := unstructured.Unstructured{Object: map[string]interface{}{}}
	raw := ar.Request.Object.Raw
	err := json.Unmarshal(raw, &obj)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	err = v.validateObj(v.crdSchemas[obj.GroupVersionKind()].Schema.OpenAPIV3Schema, obj.Object)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}

func (v *formatValidators) validateObj(schema *apiextensionsv1.JSONSchemaProps, obj interface{}) error {
	klog.V(2).Info("calling validateObj on object")
	if len(schema.Format) > 0 {
		parts := strings.SplitN(schema.Format, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("expected format of the form <FormatValidator-id>:<FormatValidator-specific-content>, but got %s", schema.Format)
		}
		validatorId := parts[0]
		validatorSpecificContent := parts[1]
		validator, ok := v.validators[validatorId]
		if !ok {
			return nil // ignore unsupported validators
		}
		klog.V(2).Infof("calling %s validator", validatorId)
		if err := validator.Validate(validatorSpecificContent, obj); err != nil {
			return err
		}
	}
	if len(schema.Properties) > 0 {
		if m, ok := obj.(map[string]interface{}); ok { // TODO: should return error if not
			for prop, s := range schema.Properties {
				klog.V(2).Infof("property: %s", prop)
				if propObj, ok := m[prop]; ok {
					klog.V(2).Infof("property value: %v", propObj)
					if err := v.validateObj(&s, propObj); err != nil {
						return err
					}

				}
			}
		}
	}
	return nil
}