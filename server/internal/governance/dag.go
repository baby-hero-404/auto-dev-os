package governance

import "fmt"

// StepSpec is the minimal shape needed to structurally validate a pipeline
// graph: an id and the ids it depends on.
type StepSpec struct {
	ID        string
	DependsOn []string
}

// ValidationError is one structural or schema problem found in a config,
// reported with a JSON-pointer-ish path so a UI can show it inline (REQ-001).
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidateDAG runs the five structural checks design.md requires for a full
// custom pipeline graph (REQ-001b): deps resolve, acyclic, exactly one
// entry, every node reachable from the entry, and every node has a path to
// some terminal (leaf) node. It returns all violations found (not just the
// first), since the UI shows a list of errors.
func ValidateDAG(steps []StepSpec) []ValidationError {
	var errs []ValidationError
	if len(steps) == 0 {
		return errs
	}

	byID := make(map[string]StepSpec, len(steps))
	for _, s := range steps {
		byID[s.ID] = s
	}

	errs = append(errs, checkDepsResolve(steps, byID)...)
	// Cycles or unresolved deps make entry/reachability/dead-end analysis
	// unreliable — skip the rest of the checks so we don't pile on
	// confusing derivative errors, mirroring how a compiler stops with
	// e.g. "undefined step" rather than also complaining every user of it
	// is unreachable.
	if len(errs) > 0 {
		return errs
	}

	if cyc := checkAcyclic(steps, byID); cyc != nil {
		errs = append(errs, *cyc)
		return errs
	}

	entries := findEntries(steps)
	errs = append(errs, checkSingleEntry(entries)...)
	if len(entries) != 1 {
		return errs
	}

	reachable := reachableFrom(entries[0], byID)
	errs = append(errs, checkReachability(steps, reachable)...)

	terminals := findTerminals(steps)
	errs = append(errs, checkNoDeadEnds(steps, byID, terminals)...)

	return errs
}

func checkDepsResolve(steps []StepSpec, byID map[string]StepSpec) []ValidationError {
	var errs []ValidationError
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := byID[dep]; !ok {
				errs = append(errs, ValidationError{
					Path:    fmt.Sprintf("pipeline.steps[%s].dependsOn", s.ID),
					Message: fmt.Sprintf("unresolved dependency: %q", dep),
				})
			}
		}
	}
	return errs
}

func checkAcyclic(steps []StepSpec, byID map[string]StepSpec) *ValidationError {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(steps))
	var cycleStep string

	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		for _, dep := range byID[id].DependsOn {
			switch color[dep] {
			case gray:
				cycleStep = id
				return true
			case white:
				if visit(dep) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}

	for _, s := range steps {
		if color[s.ID] == white {
			if visit(s.ID) {
				return &ValidationError{
					Path:    fmt.Sprintf("pipeline.steps[%s]", cycleStep),
					Message: "cycle detected in pipeline dependsOn graph",
				}
			}
		}
	}
	return nil
}

func findEntries(steps []StepSpec) []string {
	var entries []string
	for _, s := range steps {
		if len(s.DependsOn) == 0 {
			entries = append(entries, s.ID)
		}
	}
	return entries
}

func checkSingleEntry(entries []string) []ValidationError {
	if len(entries) == 1 {
		return nil
	}
	if len(entries) == 0 {
		return []ValidationError{{Path: "pipeline.steps", Message: "no entry step found: every step has a dependsOn"}}
	}
	return []ValidationError{{Path: "pipeline.steps", Message: fmt.Sprintf("multiple entry steps found (must have exactly 1): %v", entries)}}
}

func reachableFrom(entry string, byID map[string]StepSpec) map[string]bool {
	// DependsOn points from a step to its prerequisites, so "reachable from
	// entry" means traversing the reverse edges (entry -> steps that depend
	// on it, transitively). Build a forward adjacency (dependency -> dependents)
	// first, then BFS from entry.
	dependents := make(map[string][]string)
	for id, s := range byID {
		for _, dep := range s.DependsOn {
			dependents[dep] = append(dependents[dep], id)
		}
	}

	seen := map[string]bool{entry: true}
	queue := []string{entry}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range dependents[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return seen
}

func checkReachability(steps []StepSpec, reachable map[string]bool) []ValidationError {
	var errs []ValidationError
	for _, s := range steps {
		if !reachable[s.ID] {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("pipeline.steps[%s]", s.ID),
				Message: fmt.Sprintf("unreachable step: %s", s.ID),
			})
		}
	}
	return errs
}

func findTerminals(steps []StepSpec) map[string]bool {
	hasDependent := make(map[string]bool)
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			hasDependent[dep] = true
		}
	}
	terminals := make(map[string]bool)
	for _, s := range steps {
		if !hasDependent[s.ID] {
			terminals[s.ID] = true
		}
	}
	return terminals
}

func checkNoDeadEnds(steps []StepSpec, byID map[string]StepSpec, terminals map[string]bool) []ValidationError {
	// reachesTerminal(id): does id have a path (via steps that depend on it)
	// to some terminal node. Computed as reverse-reachability from the set
	// of terminals, walking DependsOn edges backward from each terminal.
	canReach := make(map[string]bool)
	var mark func(id string)
	mark = func(id string) {
		if canReach[id] {
			return
		}
		canReach[id] = true
		for _, dep := range byID[id].DependsOn {
			mark(dep)
		}
	}
	for t := range terminals {
		mark(t)
	}

	var errs []ValidationError
	for _, s := range steps {
		if !canReach[s.ID] {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("pipeline.steps[%s]", s.ID),
				Message: fmt.Sprintf("dead-end step: %s has no path to a terminal step", s.ID),
			})
		}
	}
	return errs
}
