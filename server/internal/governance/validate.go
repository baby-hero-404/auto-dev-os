package governance

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schemas/pipeline.schema.json schemas/policies.schema.json
var schemaFS embed.FS

var (
	pipelineSchema *jsonschema.Schema
	policiesSchema *jsonschema.Schema
)

func init() {
	pipelineSchema = mustCompile("schemas/pipeline.schema.json")
	policiesSchema = mustCompile("schemas/policies.schema.json")
}

func mustCompile(path string) *jsonschema.Schema {
	raw, err := schemaFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("governance: failed to read embedded schema %s: %v", path, err))
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(path, bytes.NewReader(raw)); err != nil {
		panic(fmt.Sprintf("governance: failed to add schema resource %s: %v", path, err))
	}
	s, err := c.Compile(path)
	if err != nil {
		panic(fmt.Sprintf("governance: failed to compile schema %s: %v", path, err))
	}
	return s
}

// ValidateConfig schema-validates raw (a project's pipeline_config JSON
// column) and, when the pipeline section declares a full custom step graph
// (every step carries its own dependsOn — see design.md's "extends: null"
// case), runs the DAG structural checks from dag.go. It returns the parsed
// Config on success. A nil/empty raw is not an error — callers should treat
// that as "no config" (REQ-002) without calling ValidateConfig at all.
func ValidateConfig(raw []byte) (*Config, []ValidationError, error) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, []ValidationError{{Path: "$", Message: "invalid JSON: " + err.Error()}}, nil
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, []ValidationError{{Path: "$", Message: "invalid JSON: " + err.Error()}}, nil
	}

	var errs []ValidationError

	if cfg.Version != CurrentVersion {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: fmt.Sprintf("unsupported config version %d: this build understands version %d only", cfg.Version, CurrentVersion),
		})
	}

	if pipelineRaw, ok := generic["pipeline"]; ok {
		errs = append(errs, validateAgainstSchema(pipelineSchema, "pipeline", pipelineRaw)...)
	}
	if policiesRaw, ok := generic["policies"]; ok {
		errs = append(errs, validateAgainstSchema(policiesSchema, "policies", policiesRaw)...)
	}

	// Only run DAG structural checks when the user declared a full custom
	// graph (every step lists its own dependsOn explicitly). Patch-mode
	// configs (extends + a handful of enabled/skip_when overrides against
	// steps from the registry) aren't a standalone graph and have nothing
	// to structurally validate here.
	if cfg.Pipeline != nil && isFullCustomGraph(cfg.Pipeline) {
		specs := make([]StepSpec, len(cfg.Pipeline.Steps))
		for i, s := range cfg.Pipeline.Steps {
			specs[i] = StepSpec{ID: s.ID, DependsOn: s.DependsOn}
		}
		errs = append(errs, ValidateDAG(specs)...)
	}

	if len(errs) > 0 {
		return nil, errs, nil
	}
	return &cfg, nil, nil
}

func isFullCustomGraph(p *Pipeline) bool {
	if p.Extends != "" || len(p.Steps) == 0 {
		return false
	}
	for _, s := range p.Steps {
		if s.DependsOn == nil {
			return false
		}
	}
	return true
}

func validateAgainstSchema(schema *jsonschema.Schema, prefix string, raw json.RawMessage) []ValidationError {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return []ValidationError{{Path: prefix, Message: "invalid JSON: " + err.Error()}}
	}
	if err := schema.Validate(v); err != nil {
		if verr, ok := err.(*jsonschema.ValidationError); ok {
			return flattenSchemaErrors(prefix, verr)
		}
		return []ValidationError{{Path: prefix, Message: err.Error()}}
	}
	return nil
}

func flattenSchemaErrors(prefix string, verr *jsonschema.ValidationError) []ValidationError {
	var out []ValidationError
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			path := prefix
			if e.InstanceLocation != "" {
				path = prefix + e.InstanceLocation
			}
			out = append(out, ValidationError{Path: path, Message: e.Message})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(verr)
	return out
}
