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

func validateExecutionPolicy(req CommandRequest) error {
	networkMode := req.NetworkMode
	if networkMode == NetworkModeDefault {
		networkMode = NetworkModeNone
	}
	switch networkMode {
	case NetworkModeNone, NetworkModeBridge:
	default:
		return fmt.Errorf("unsupported sandbox network mode %q", networkMode)
	}
	if networkMode != NetworkModeNone && len(req.SecretEnv) > 0 {
		return fmt.Errorf("sandbox policy blocks injecting secrets when network egress is enabled")
	}
	return nil
}
