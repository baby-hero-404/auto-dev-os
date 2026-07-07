package policy

import (
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// CalculateUnitCost calculates the execution cost of a single unit.
// It applies base costs based on keywords in the tasks, and adjusts by the risk multiplier.
func CalculateUnitCost(unit models.ExecutionUnit) float64 {
	var totalBaseCost float64

	for _, task := range unit.Tasks {
		lower := strings.ToLower(task)
		taskCost := 1.0 // Default to Modify/General task cost

		switch {
		case strings.Contains(lower, "migration") || strings.Contains(lower, "db schema") || strings.Contains(lower, "database schema"):
			taskCost = 5.0
		case strings.Contains(lower, "config") || strings.Contains(lower, "go.mod") || strings.Contains(lower, "package.json") || strings.Contains(lower, "dependency") || strings.Contains(lower, "dependencies"):
			taskCost = 3.0
		case strings.Contains(lower, "create") || strings.Contains(lower, "add ") || strings.Contains(lower, "new "):
			taskCost = 2.0
		case strings.Contains(lower, "test") || strings.Contains(lower, "spec") || strings.Contains(lower, "mock"):
			taskCost = 1.0
		case strings.Contains(lower, "modify") || strings.Contains(lower, "update") || strings.Contains(lower, "edit") || strings.Contains(lower, "refactor") || strings.Contains(lower, "fix"):
			taskCost = 1.0
		}
		totalBaseCost += taskCost
	}

	// Add file count weight if estimated
	if unit.Constraints.MaxFiles > 0 {
		totalBaseCost += float64(unit.Constraints.MaxFiles) * 0.5
	}

	// Apply Risk Multiplier
	multiplier := 1.0
	switch strings.ToLower(unit.Constraints.MaxRisk) {
	case "medium":
		multiplier = 1.2
	case "high", "critical":
		multiplier = 1.5
	}

	return totalBaseCost * multiplier
}

// ValidateDAG checks the execution units for cyclic dependencies and phase cost threshold violations.
func ValidateDAG(units []models.ExecutionUnit, maxCostThreshold float64) error {
	if len(units) == 0 {
		return nil
	}

	// 1. Check Phase Cost threshold
	for _, unit := range units {
		cost := CalculateUnitCost(unit)
		if cost > maxCostThreshold {
			return fmt.Errorf("execution unit %q exceeds max complexity threshold (cost: %.2f, max: %.2f). Please split the tasks", unit.ID, cost, maxCostThreshold)
		}
	}

	// 2. Check for Cyclic Dependencies
	adj := make(map[string][]string)
	allNodes := make(map[string]bool)

	for _, unit := range units {
		allNodes[unit.ID] = true
		for _, dep := range unit.Dependencies {
			adj[unit.ID] = append(adj[unit.ID], dep)
		}
	}

	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = 1
		for _, neighbor := range adj[node] {
			if !allNodes[neighbor] {
				// Dependency node doesn't exist in the phase list (might be an external task or draft), skip cyclic check for it
				continue
			}
			if visited[neighbor] == 1 {
				return true // Cycle detected
			}
			if visited[neighbor] == 0 {
				if dfs(neighbor) {
					return true
				}
			}
		}
		visited[node] = 2
		return false
	}

	for node := range allNodes {
		if visited[node] == 0 {
			if dfs(node) {
				return fmt.Errorf("cyclic dependency detected in execution plan starting from unit %q", node)
			}
		}
	}

	return nil
}
