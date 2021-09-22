package validators

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type FormatValidator interface {
	Validate(fieldpath []string, validatorContent string, schema *apiextensionsv1.JSONSchemaProps, obj interface{}) error
	ValidateProgram(fieldpath []string, validatorContent string, schema *apiextensionsv1.JSONSchemaProps) error
}

type Converter interface {
	Convert(fieldpath []string, validatorContent string, currentVersion, targetVersion string, currentSchema, targetSchema *apiextensionsv1.JSONSchemaProps, obj interface{}) (interface{}, error)
}
