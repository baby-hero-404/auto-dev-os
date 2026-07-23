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
	blocked := []string{"rm -rf", "mkfs", ":(){", "dd if="}
	for _, pattern := range blocked {
		if strings.Contains(joined, pattern) {
			return fmt.Errorf("sandbox policy blocked command containing %q", pattern)
		}
	}
	return nil
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
