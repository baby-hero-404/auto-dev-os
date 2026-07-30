package sandbox

import (
	"fmt"
	"strings"
)

func validateCommand(command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("sandbox command is required")
	}
	joined := strings.ToLower(strings.Join(command, " "))
	blocked := []string{"mkfs", ":(){", "dd if="}
	for _, pattern := range blocked {
		if strings.Contains(joined, pattern) {
			return fmt.Errorf("sandbox policy blocked command containing %q", pattern)
		}
	}
	if isDestructiveRm(joined) {
		return fmt.Errorf("sandbox policy blocked destructive rm command")
	}
	return nil
}

func isDestructiveRm(joined string) bool {
	if strings.Contains(joined, "--no-preserve-root") {
		return true
	}
	rmIdx := strings.Index(joined, "rm ")
	if rmIdx < 0 {
		return false
	}

	destructiveTargets := []string{
		"/",
		"/*",
		"/workspace",
		"/workspace/",
		"/workspace/*",
		"~",
		"~/",
		"~/*",
		"$home",
		"$home/",
		"$home/*",
	}

	for _, target := range destructiveTargets {
		patterns := []string{
			"rm -rf " + target,
			"rm -r " + target,
			"rm -f -r " + target,
			"rm -r -f " + target,
			"rm -rf '" + target + "'",
			"rm -rf \"" + target + "\"",
			"rm -r '" + target + "'",
			"rm -r \"" + target + "\"",
		}
		for _, p := range patterns {
			idx := 0
			for {
				found := strings.Index(joined[idx:], p)
				if found < 0 {
					break
				}
				matchPos := idx + found
				endIdx := matchPos + len(p)
				if endIdx == len(joined) {
					return true
				}
				nextChar := joined[endIdx]
				if nextChar == ' ' || nextChar == ';' || nextChar == '&' || nextChar == '|' || nextChar == '\n' || nextChar == '"' || nextChar == '\'' || nextChar == ')' || nextChar == '}' {
					return true
				}
				idx = matchPos + 1
			}
		}
	}
	return false
}

// validateExecutionPolicy enforces the secrets/network-egress exclusion.
// resolvedNetworkMode must be the mode the runtime will actually apply to
// the container (after resolving NetworkModeDefault against runtime config),
// not the raw requested mode — the two previously diverged, letting secrets
// be injected into containers that ended up with network access.
func validateExecutionPolicy(req CommandRequest, resolvedNetworkMode string) error {
	switch req.NetworkMode {
	case NetworkModeDefault, NetworkModeNone, NetworkModeBridge:
	default:
		return fmt.Errorf("unsupported sandbox network mode %q", req.NetworkMode)
	}
	if resolvedNetworkMode != NetworkModeNone && len(req.SecretEnv) > 0 {
		return fmt.Errorf("sandbox policy blocks injecting secrets when network egress is enabled")
	}
	return nil
}
