package validators

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	celext "github.com/google/cel-go/ext"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// https://github.com/tektoncd/triggers/blob/7c165fc7f5f54d6925672ff5061957b5e224135d/pkg/interceptors/cel/cel.go#L71
type CelValidator struct {
	//crdPrograms map[schema.GroupVersionKind]*gvkCelInfo
}

//type gvkCelInfo struct {
//	rootType *expr.Type
//}

func NewCelValidator() *CelValidator {
	v := &CelValidator{}
	//v.crdPrograms = map[schema.GroupVersionKind]*gvkCelInfo{}
	return v
}

//func (v *CelValidator) RegisterCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition) {
//	for _, version :=range crd.Spec.Versions {
//		gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}
//		v.crdPrograms[gvk] = &gvkCelInfo{
//
//		}
//	}
//}

func (v *CelValidator) Validate(fieldpath []string, celCode string, obj interface{}) error {
	var celDecls []*expr.Decl
	celVars := map[string]interface{}{}
	switch o := obj.(type) {
	case map[string]interface{}:
		for k, v := range o {
			// TODO: recursively traverse instead of nesting this 1 level deep
			// TODO: fully declare types instead of using Dyn
			// TODO: support all types
			switch vt := v.(type) {
			case string:
				celDecls = append(celDecls, decls.NewVar(k, decls.String))
				celVars[k] = vt
			case int32, int64:
				celDecls = append(celDecls, decls.NewVar(k, decls.Int))
				celVars[k] = vt
			case map[string]interface{}:
				celDecls = append(celDecls, decls.NewVar(k, decls.NewMapType(decls.String, decls.Dyn)))
				celVars[k] = types.NewDynamicMap(types.DefaultTypeAdapter, v)
			}
		}
	case string:
		fieldName := fieldpath[len(fieldpath)-1]
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.String))
		celVars[fieldName] = o
	case int32, int64:
		fieldName := fieldpath[len(fieldpath)-1]
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.Int))
		celVars[fieldName] = o
	default:
		// TODO: pass in the actual field name as the root var?
	}

	env, err := cel.NewEnv(
		celext.Strings(),
		celext.Encoders(),
		cel.Declarations(celDecls...))
	if err != nil {
		return fmt.Errorf("error initializing CEL environment: %w", err)
	}
	ast, issues := env.Compile(celCode)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL compile error: %w", issues.Err())
	}
	ast, issues = env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL type-check error: %w", issues.Err())
	}
	// TODO: this program is cachable and absolutely should be cached
	prg, err := env.Program(ast)
	if err != nil {
		return fmt.Errorf("CEL program construction error: %w", err)
	}

	out, _, err := prg.Eval(celVars)
	if err != nil {
		return fmt.Errorf("CEL program evaluation error: %w for: %#+v, cel: %s", err, obj, celCode)
	}
	if out.Value()  != true {
		// TODO: Will need much better error reporting here
		return fmt.Errorf("CEL format expression evaluated to false: %s", celCode)
	}
	return nil
}
