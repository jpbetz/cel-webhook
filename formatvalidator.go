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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"

	"github.com/jpbetz/runner-webhook/validators"
)

type RegisterAware interface {
	RegisterCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition)
}

type formatValidators struct {
	validators map[string]validators.FormatValidator
	converters map[string]validators.Converter
	crdSchemas map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion
}

func newFormatValidators() *formatValidators {
	v := &formatValidators{}
	v.validators = map[string]validators.FormatValidator{}
	v.converters = map[string]validators.Converter{}
	v.crdSchemas = map[schema.GroupVersionKind]*apiextensionsv1.CustomResourceDefinitionVersion{}
	return v
}

func (v *formatValidators) RegisterCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition) {
	for _, version := range crd.Spec.Versions {
		gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}
		cp := version
		v.crdSchemas[gvk] = &cp
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

func (v *formatValidators) registerConverter(converterId string, converter validators.Converter) {
	v.converters[converterId] = converter
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
	serve(w, r, v.validateRequest, v.convertRequest)
}

func (v *formatValidators) convertRequest(convertRequest apiextensionsv1.ConversionReview) *apiextensionsv1.ConversionResponse {
	var convertedObjects []runtime.RawExtension
	for _, obj := range convertRequest.Request.Objects {
		cr := unstructured.Unstructured{}
		if err := cr.UnmarshalJSON(obj.Raw); err != nil {
			return &apiextensionsv1.ConversionResponse{
				Result: metav1.Status{Status: metav1.StatusFailure, Message: err.Error()}, // TODO: distinguish between client and server errors
			}
		}

		currentGVK := cr.GroupVersionKind()
		targetGVK := schema.FromAPIVersionAndKind(convertRequest.Request.DesiredAPIVersion, currentGVK.Kind)
		if targetCrd, ok := v.crdSchemas[targetGVK]; ok {
			if currentCrd, ok := v.crdSchemas[currentGVK]; ok {
				klog.Infof("converting from %v to %v (%s to %s)", currentGVK, targetGVK, currentCrd.Name, targetCrd.Name)
				converted, err := v.convertObj(nil, currentCrd.Schema.OpenAPIV3Schema, targetCrd.Schema.OpenAPIV3Schema, cr.Object)
				if err != nil {
					return &apiextensionsv1.ConversionResponse{
						Result: metav1.Status{Status: metav1.StatusFailure, Message: err.Error()}, // TODO: distinguish between client and server errors
					}
				}
				out, ok := converted.(map[string]interface{})
				if !ok {
					return &apiextensionsv1.ConversionResponse{
						Result: metav1.Status{Reason: metav1.StatusFailure, Message: fmt.Sprintf("Expected map in conversion response but got %T", converted)},
					}
				}
				convertedCR := &unstructured.Unstructured{Object: out}
				convertedCR.SetAPIVersion(convertRequest.Request.DesiredAPIVersion)
				convertedCR.SetKind(cr.GetKind())

				convertedCR.SetName(cr.GetName())
				convertedCR.SetGenerateName(cr.GetGenerateName())
				convertedCR.SetNamespace(cr.GetNamespace())
				convertedCR.SetSelfLink(cr.GetSelfLink())
				convertedCR.SetUID(cr.GetUID())
				convertedCR.SetResourceVersion(cr.GetResourceVersion())
				convertedCR.SetGeneration(cr.GetGeneration())
				convertedCR.SetNamespace(cr.GetNamespace())
				convertedCR.SetCreationTimestamp(cr.GetCreationTimestamp())
				convertedCR.SetDeletionTimestamp(cr.GetDeletionTimestamp())
				convertedCR.SetDeletionGracePeriodSeconds(cr.GetDeletionGracePeriodSeconds())
				convertedCR.SetLabels(cr.GetLabels())
				convertedCR.SetAnnotations(cr.GetAnnotations())
				convertedCR.SetOwnerReferences(cr.GetOwnerReferences())
				convertedCR.SetFinalizers(cr.GetFinalizers())
				convertedCR.SetClusterName(cr.GetClusterName())
				convertedCR.SetManagedFields(cr.GetManagedFields())

				// TODO: handle all object meta
				convertedObjects = append(convertedObjects, runtime.RawExtension{Object: convertedCR})
			}
		}
	}
	return &apiextensionsv1.ConversionResponse{
		ConvertedObjects: convertedObjects,
		Result: metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}
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
		if validator, ok := v.validators[validatorId]; ok {
			if err := validator.ValidateProgram(fieldpath, validatorSpecificContent, schema); err != nil {
				return err
			}
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

func (v *formatValidators) convertObj(fieldpath []string, currentSchema, targetSchema *apiextensionsv1.JSONSchemaProps, obj interface{}) (interface{}, error) {
	klog.Infof("Conversion requested for path %v, format: %s", fieldpath, targetSchema.Format)
	if len(targetSchema.Format) > 0 {
		parts := strings.SplitN(targetSchema.Format, ":", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("expected format of the form <id>:<content>, but got %s", targetSchema.Format)
		}
		id := parts[0]
		code := parts[1]
		converter, ok := v.converters[id]
		if ok {
			return converter.Convert(fieldpath, code, currentSchema, targetSchema, obj)
		}
	}
	if len(targetSchema.Properties) > 0 {
		if in, ok := obj.(map[string]interface{}); ok {
			mout := map[string]interface{}{}
			// TODO: walk both schema here
			// TODO: only automatically convert things to the new schema when type and fieldname match
			for propName, prop := range targetSchema.Properties {
				currentProp := currentSchema.Properties[propName]
				if value, ok := in[propName]; ok {
					out, err := v.convertObj(append(fieldpath, propName), &currentProp, &prop, value)
					if err != nil {
						return nil, err
					}
					mout[propName] = out
				}
			}
			klog.Info("Converter: returning converted map")
			return mout, nil
		} else {
			klog.Info("Converter: object was not map")
		}
	}
	return obj, nil
}
