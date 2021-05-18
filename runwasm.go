package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	wasmer "github.com/wasmerio/wasmer-go/wasmer"
	"k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
)

type validator struct {
	// TODO: Should probably not leave modules running?
	moduleInstances map[string]*wasmer.Instance
	crdSchemas map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion
}

func newValidator() *validator {
	v := &validator{}
	v.moduleInstances = map[string]*wasmer.Instance{}
	v.crdSchemas = map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion{}
	return v
}

func (v *validator) registerCrd(filepath string) error {
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

func (v *validator) registerModule(filepath string) error {
	b, err := os.ReadFile(filepath)
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	// Compiles the module
	module, err := wasmer.NewModule(store, b)
	if err != nil {
		return fmt.Errorf("error creating module: %w", err)
	}

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		return fmt.Errorf("error createing module instance: %w", err)
	}
	v.moduleInstances[filepath] = instance
	return nil
}

func (v *validator) validateObject(module, functionName string, obj interface{}) error {
	instance := v.moduleInstances[module]

	// Gets the exported function from the WebAssembly instance.
	fn, err := instance.Exports.GetFunction(functionName)
	if err != nil {
		return fmt.Errorf("error getting exported function %s: %w", functionName, err)
	}
	result, err := fn(obj)
	if err != nil {
		return fmt.Errorf("error calling exported function %s: %w", functionName, err)
	}

	klog.Error(result)
	return nil
}

func (v *validator) loadCrd(crdfilepath string) (*apiextensionsv1.CustomResourceDefinition, error) {
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

func (v *validator) serveValidate(w http.ResponseWriter, r *http.Request) {
	serve(w, r, v.validateWasm)
}

func (v *validator) validateWasm(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.V(2).Info("calling run WebAssembly webhook")
	obj := unstructured.Unstructured{Object: map[string]interface{}{}}
	raw := ar.Request.Object.Raw
	err := json.Unmarshal(raw, &obj)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	err = v.validate(v.crdSchemas[obj.GroupVersionKind()].Schema.OpenAPIV3Schema, obj.Object)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}

func (v *validator) validate(schema *apiextensionsv1.JSONSchemaProps, obj interface{}) error {
	klog.V(2).Info("calling validate on object")
	if len(schema.Format) > 0 && strings.HasPrefix(schema.Format, "wasm:") {
		parts := strings.Split(schema.Format, ":")
		if len(parts) < 3 {
			return fmt.Errorf("expected format of the form wasm:<module>:<function>, but got %s", schema.Format)
		}
		module := parts[1]
		function := parts[2]
		klog.V(2).Infof("calling validateObject on module:%s, function: %s", module, function)
		if err := v.validateObject(module, function, obj); err != nil {
			return err
		}
	}
	if len(schema.Properties) > 0 {
		if m, ok := obj.(map[string]interface{}); ok { // TODO: should return error if not
			for prop, s := range schema.Properties {
				klog.V(2).Infof("property: %s", prop)
				if propObj, ok := m[prop]; ok {
					klog.V(2).Infof("property value: %v", propObj)
					if err := v.validate(&s, propObj); err != nil {
						return err
					}

				}
			}
		}
	}
	return nil
}