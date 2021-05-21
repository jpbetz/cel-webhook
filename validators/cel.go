package validators

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	celext "github.com/google/cel-go/ext"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	if program, ok := v.compiledPrograms[programPath]; ok {
		return program, nil
	}

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
		return nil, fmt.Errorf("validation rule compile error: %w", issues.Err())
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

	var root string
	if len(fieldpath) > 0 {
		root = fieldpath[len(fieldpath)-1]
	}  else {
		root = "object"
	}
	celVars := map[string]interface{}{}
	v.buildVars([]string{root}, obj, celVars)
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