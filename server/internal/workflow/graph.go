package workflow

// graph.go provides graph utilities for workflow DAG validation.
// The actual DAG definition lives in steps.go (DefaultWorkflowDefinition).
// This file contains the topological sort cycle detector used to
// validate definitions at startup.

import "fmt"

// ValidateDAG runs a topological sort on a Definition to detect cycles.
// Returns the sorted step IDs or an error if a cycle exists.
func ValidateDAG(def Definition) ([]string, error) {
	inDeg := map[string]int{}
	deps := map[string][]string{}
	for _, s := range def.Steps {
		if _, ok := inDeg[s.ID]; !ok {
			inDeg[s.ID] = 0
		}
		for _, d := range s.DependsOn {
			inDeg[s.ID]++
			deps[d] = append(deps[d], s.ID)
		}
	}

	var queue []string
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, child := range deps[cur] {
			inDeg[child]--
			if inDeg[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(order) != len(inDeg) {
		return nil, fmt.Errorf("cycle detected in workflow graph")
	}
	return order, nil
}
