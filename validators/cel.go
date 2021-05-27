package validators

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	celext "github.com/google/cel-go/ext"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/klog"
)

type CelValidator struct {
	compiledPrograms map[string]cel.Program
}

func NewCelValidator() *CelValidator {
	v := &CelValidator{}
	v.compiledPrograms = map[string]cel.Program{}
	return v
}

func (v *CelValidator) compileProgram(fieldpath []string, celSource string, schema *apiextensionsv1.JSONSchemaProps) (cel.Program, error) {
	programPath := "/" + strings.Join(fieldpath, "/")
	// TODO: reenable once program caching is scoped to crd and invalidation is in place for crd reloads
	//if program, ok := v.compiledPrograms[programPath]; ok {
	//	return program, nil
	//}

	var root string
	if len(fieldpath) > 0 {
		root = fieldpath[len(fieldpath)-1]
	}  else {
		root = "object"
	}
	celDecls := v.buildDecl([]string{root}, schema)
	env, err := cel.NewEnv(
		celext.Strings(),
		celext.Encoders(),
		cel.Declarations(celDecls...))
	if err != nil {
		return nil, fmt.Errorf("error initializing CEL environment: %w", err)
	}
	ast, issues := env.Compile(celSource)
	if issues != nil && issues.Err() != nil {
		var decls string
		for _, decl := range celDecls {
			decls += fmt.Sprintf("(name: %s)", decl.Name)

		}
		return nil, fmt.Errorf("compile error for buildDecls %s: %w", decls, issues.Err())
	}
	ast, issues = env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL type-check error: %w", issues.Err())
	}
	// TODO: this program is cachable and absolutely should be cached
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program construction error: %w", err)
	}
	v.compiledPrograms[programPath] = prg
	return prg, nil
}

func (v *CelValidator) buildDecl(fieldpath []string, schema *apiextensionsv1.JSONSchemaProps) []*expr.Decl {
	var celDecls []*expr.Decl
	switch schema.Type {
	case "object":
		for k, prop := range schema.Properties {
			celDecls = append(celDecls, v.buildDecl(append(fieldpath, k), &prop)...)
		}
	case "array":
		// TODO support properly with type checking
		fieldName := strings.Join(fieldpath, ".")
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.NewListType(decls.Dyn)))
	case "string":
		fieldName := strings.Join(fieldpath, ".")
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.String))
	case "integer":
		fieldName := strings.Join(fieldpath, ".")
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.Int))
	case "number":
		fieldName := strings.Join(fieldpath, ".")
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.Double))
	case "boolean":
		fieldName := strings.Join(fieldpath, ".")
		celDecls = append(celDecls, decls.NewVar(fieldName, decls.Bool))
	default:
		// TODO: handle as error
	}
	return celDecls
}

func (v *CelValidator) ValidateProgram(fieldpath []string, validatorContent string, schema *apiextensionsv1.JSONSchemaProps) error {
	_, err := v.compileProgram(fieldpath, validatorContent, schema)
	return err
}

func (v *CelValidator) Validate(fieldpath []string, celSource string, schema *apiextensionsv1.JSONSchemaProps, obj interface{}) error {
	prg, err := v.compileProgram(fieldpath, celSource, schema)
	if err != nil {
		return fmt.Errorf("validation rule compile error: %w for: %#+v, rule: %s", err, obj, celSource)
	}
	celVars := map[string]interface{}{}
	v.buildVars([]string{}, obj, celVars)
	out, _, err := prg.Eval(celVars)
	if err != nil {
		return fmt.Errorf("validation rule evaluation error: %w for: %#+v, rule: %s", err, obj, celSource)
	}
	if out.Value()  != true {
		// TODO: Will need much better error reporting here
		return fmt.Errorf("validation failed for: %s", celSource)
	}
	return nil
}

func (v *CelValidator) buildVars(fieldpath []string, obj interface{}, celVars map[string]interface{}) {
	switch objVal := obj.(type) {
	case map[string]interface{}:
		for k, value := range objVal {
			v.buildVars(append(fieldpath, k), value, celVars)
		}
	case []interface{}:
		// TODO: handled in structured way
		fieldName := strings.Join(fieldpath, ".")
		celVars[fieldName] = obj
	default:
		fieldName := strings.Join(fieldpath, ".")
		celVars[fieldName] = obj
	}
}


// TODO: will probably need to walk both the old and new schemas
// to support mapping rules like: from(v1): new.newfieldname := old.oldfieldname
func (v *CelValidator) Convert(fieldpath []string, celSource string, currentVersion, targetVersion string, currentSchema, targetSchema *apiextensionsv1.JSONSchemaProps, obj interface{}) (interface{}, error) {
	klog.Infof("Running converter: %s on %v", celSource, fieldpath)
	prg, err := v.compileProgram([]string{"v1"}, celSource, currentSchema) // Schema is expected to be the old schema (for now)
	if err != nil {
		return nil, fmt.Errorf("conversion rule compile error: %w for: %#+v, rule: %s", err, obj, celSource)
	}
	celVars := map[string]interface{}{}
	v.buildVars([]string{"v1"}, obj, celVars)
	out, _, err := prg.Eval(celVars)
	if err != nil {
		return nil, fmt.Errorf("conversion rule evaluation error: %w for: %#+v, celVars: %#+v, rule: %s", err, obj, celVars, celSource)
	}
	result, err := out.ConvertToNative(reflect.TypeOf(obj))
	if err != nil {
		return nil, err
	}

	// basic automatic conversion of things not manually converted
	if m, ok := obj.(map[string]interface{}); ok {
		resultm := result.(map[string]interface{})
		for k, v := range m {
			if _, ok = resultm[k]; !ok {
				resultm[k] = v
			}
		}
	}
	return result, nil
}
