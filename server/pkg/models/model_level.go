package models

// Model levels are the abstract capability tier an agent or project asks for
// ("give me a fast model" / "give me a powerful one"), independent of any
// particular model name.
//
// These outlived the provider_models catalog they used to sit beside. That
// catalog mapped each level to concrete model names per API provider, and died
// with the LLM API layer — a CLI harness picks its own model. The levels stay
// because agent and project config still carry them as an intent signal.
const (
	ModelLevelFast     = "fast"
	ModelLevelBalanced = "balanced"
	ModelLevelPowerful = "powerful"
)
