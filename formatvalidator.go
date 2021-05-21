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

type RegisterAware interface {
	RegisterCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition)
}

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

func (v *formatValidators) RegisterCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition) {
	for _, version := range crd.Spec.Versions {
		gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}
		v.crdSchemas[gvk] = &version
	}
	for _, validator := range v.validators {
		if r, ok := validator.(RegisterAware); ok {
			r.RegisterCustomResourceDefinition(crd)
		}
	}
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
	if ar.Request.Kind.String() == apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition").String() {
		crd := apiextensionsv1.CustomResourceDefinition{}
		raw := ar.Request.Object.Raw
		err := json.Unmarshal(raw, &crd)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		for _, version := range crd.Spec.Versions {
			err = v.validatePrograms(nil, version.Schema.OpenAPIV3Schema)
			if err != nil {
				klog.Error(err)
				return toV1AdmissionResponse(err)
			}
		}
	}


	obj := unstructured.Unstructured{Object: map[string]interface{}{}}
	raw := ar.Request.Object.Raw
	err := json.Unmarshal(raw, &obj)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	if crd, ok := v.crdSchemas[obj.GroupVersionKind()]; ok {
		err = v.validateObj(nil, crd.Schema.OpenAPIV3Schema, obj.Object)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
	}

	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}

func (v *formatValidators) validatePrograms(fieldpath []string, schema *apiextensionsv1.JSONSchemaProps) error {
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
		// TODO: use real fieldpaths, i.e. structured-merge-diff ones
		if err := validator.ValidateProgram(fieldpath, validatorSpecificContent, schema); err != nil {
			return err
		}
	}
	if schema.Type == "object" {
		for propName, prop := range schema.Properties {
			if err := v.validatePrograms(append(fieldpath, propName), &prop); err != nil {
				return err
			}
		}
	}
	if schema.Type == "array" {
		if schema.Items == nil {
			return fmt.Errorf("expected items to be non-nil for array type")
		}
		if err := v.validatePrograms(append(fieldpath, "item"), schema.Items.Schema); err != nil {
			return err
		}
	}
	return nil
}

func (v *formatValidators) validateObj(fieldpath []string, schema *apiextensionsv1.JSONSchemaProps, obj interface{}) error {
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
		// TODO: use real fieldpaths, i.e. structured-merge-diff ones
		if err := validator.Validate(fieldpath, validatorSpecificContent, schema, obj); err != nil {
			return err
		}
	}
	if schema.Type == "object" {
		if m, ok := obj.(map[string]interface{}); ok { // TODO: should return error if not
			for propName, prop := range schema.Properties {
				if propObj, ok := m[propName]; ok {
					if err := v.validateObj(append(fieldpath, propName), &prop, propObj); err != nil {
						return err
					}
				}
			}
		}
	}
	if schema.Type == "array" {
		if schema.Items == nil {
			return fmt.Errorf("expected items to be non-nil for array type")
		}
		if items, ok := obj.([]interface{}); ok { // TODO: should return error if not
			for _, item := range items {
				if err := v.validateObj(append(fieldpath, "item"), schema.Items.Schema, item); err != nil {
					return err
				}
			}
		}
	}
	return nil
}