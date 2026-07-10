package tool

import "context"

// VerifyHook runs a post-edit verification step.
type VerifyHook interface {
	Name() string
	Run(ctx context.Context, workspace string, files []string) []Diagnostic
}

// VerifyPipeline chains multiple verification hooks.
type VerifyPipeline struct {
	Hooks []VerifyHook
}

// Run executes all hooks in order, returning accumulated diagnostics.
func (vp *VerifyPipeline) Run(ctx context.Context, workspace string, files []string) []Diagnostic {
	var all []Diagnostic
	for _, hook := range vp.Hooks {
		diags := hook.Run(ctx, workspace, files)
		all = append(all, diags...)
		// Stop on first error
		for _, d := range diags {
			if d.Severity == "error" {
				return all
			}
		}
	}
	return all
}
