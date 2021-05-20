package validators

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

type CelValidator struct {
}

func NewCelValidator() *CelValidator {
	v := &CelValidator{}
	return v
}

func (v *CelValidator) Validate(celCode string, obj interface{}) error {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("field", decls.Dyn))) // TODO: map to actual schema type?
	if err != nil {
		return fmt.Errorf("error initializing CEL environment: %w", err)
	}
	ast, issues := env.Compile(celCode)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL type-check error: %w", issues.Err())
	}
	// TODO: this program is cachable and absolutely should be cached
	prg, err := env.Program(ast)
	if err != nil {
		return fmt.Errorf("CEL program construction error: %w", err)
	}
	out, _, err := prg.Eval(map[string]interface{}{
		"field": obj,
	})
	if err != nil {
		return fmt.Errorf("CEL program evaluation error: %w", err)
	}
	return fmt.Errorf("cel result: %v", out.Value())
	//if out.Value()  != true {
	//	// TODO: Will need a much better error reported back here
	//	return fmt.Errorf("field failed CEL validation rule")
	//}
	//return nil
}
