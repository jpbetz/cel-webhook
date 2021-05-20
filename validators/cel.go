package validators

import (
	"fmt"

	"github.com/google/cel-go/common/types"
	celext "github.com/google/cel-go/ext"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

// https://github.com/tektoncd/triggers/blob/7c165fc7f5f54d6925672ff5061957b5e224135d/pkg/interceptors/cel/cel.go#L71
type CelValidator struct {
}

func NewCelValidator() *CelValidator {
	v := &CelValidator{}
	return v
}

func (v *CelValidator) Validate(celCode string, obj interface{}) error {
	env, err := cel.NewEnv(
		celext.Strings(),
		celext.Encoders(),
		cel.Declarations(
			// TODO: for maps, explode out all the fields so we can do "replicas < 5" instead of "field.replicas < 5"
			// TODO: for primitives, match their exact type? Or just use Dyn.
			// The more type information provided here the more CEL errors are caught at development time.
			decls.NewVar("field", decls.NewMapType(decls.String, decls.Dyn)))) // TODO: map to actual schema type?
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

	out, _, err := prg.Eval(map[string]interface{}{
		"field": types.NewDynamicMap(types.DefaultTypeAdapter, obj),
	})
	if err != nil {
		return fmt.Errorf("CEL program evaluation error: %w for: %#+v, cel: %s", err, obj, celCode)
	}
	if out.Value()  != true {
		// TODO: Will need much better error reporting here
		return fmt.Errorf("CEL format expression evaluated to false: %s", celCode)
	}
	return nil
}
