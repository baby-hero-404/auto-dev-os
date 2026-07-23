package governance

import "embed"

//go:embed presets/api_native.json presets/cli_spec_first.json
var presetFS embed.FS

// PresetNames are the built-in presets ship-able from the UI's preset
// picker (REQ-005). Each preset is just `{"extends": "<name>"}` with no
// overrides — selecting one and saving it without edits is a no-op
// equivalent to leaving pipeline_config null (REQ-002).
var PresetNames = []string{"api_native", "cli_spec_first"}

// Preset returns the raw JSON for a built-in preset by name, or an error if
// name isn't one of PresetNames.
func Preset(name string) ([]byte, error) {
	return presetFS.ReadFile("presets/" + name + ".json")
}
