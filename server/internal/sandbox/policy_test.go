package sandbox

import "testing"

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		wantErr bool
	}{
		{name: "empty", command: nil, wantErr: true},
		{name: "safe", command: []string{"go", "test", "./..."}, wantErr: false},
		{name: "blocked rm workspace", command: []string{"bash", "-lc", "rm -rf /workspace"}, wantErr: true},
		{name: "blocked rm root", command: []string{"bash", "-lc", "rm -rf /"}, wantErr: true},
		{name: "allowed subpath autocode cleanup", command: []string{"bash", "-lc", "cd '/workspace' && claude code; rm -rf '/workspace/.autocode'; exit $status"}, wantErr: false},
		{name: "allowed node_modules cleanup", command: []string{"bash", "-lc", "rm -rf node_modules"}, wantErr: false},
		{name: "blocked mkfs", command: []string{"mkfs.ext4", "/dev/sda"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCommand(tc.command)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCommand() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateExecutionPolicy(t *testing.T) {
	secrets := map[string]string{"TOKEN": "secret"}
	tests := []struct {
		name     string
		req      CommandRequest
		resolved string
		wantErr  bool
	}{
		{name: "default resolved to none", req: CommandRequest{}, resolved: NetworkModeNone, wantErr: false},
		{name: "bridge without secrets", req: CommandRequest{NetworkMode: NetworkModeBridge}, resolved: "bridge", wantErr: false},
		{name: "bridge with secrets", req: CommandRequest{NetworkMode: NetworkModeBridge, SecretEnv: secrets}, resolved: "bridge", wantErr: true},
		{name: "unsupported network", req: CommandRequest{NetworkMode: "host"}, resolved: "host", wantErr: true},
		// The mismatch the policy previously missed: NetworkModeDefault
		// requested, but the runtime resolves it to an egress-enabled mode
		// because DisableNetworking is false. Secrets must be blocked.
		{name: "default resolved to bridge with secrets", req: CommandRequest{NetworkMode: NetworkModeDefault, SecretEnv: secrets}, resolved: "bridge", wantErr: true},
		{name: "default resolved to none with secrets", req: CommandRequest{NetworkMode: NetworkModeDefault, SecretEnv: secrets}, resolved: NetworkModeNone, wantErr: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExecutionPolicy(tc.req, tc.resolved)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateExecutionPolicy() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestResolveNetworkMode(t *testing.T) {
	egressEnabled := &DockerRuntime{config: DockerConfig{DisableNetworking: false}}
	egressDisabled := &DockerRuntime{config: DockerConfig{DisableNetworking: true}}

	if got := egressEnabled.resolveNetworkMode(NetworkModeDefault); got != "bridge" {
		t.Errorf("default with egress enabled: expected bridge (not host — see sandbox-network-isolation-fix), got %q", got)
	}
	if got := egressDisabled.resolveNetworkMode(NetworkModeDefault); got != NetworkModeNone {
		t.Errorf("default with egress disabled: expected none, got %q", got)
	}
	// Bridge is explicitly requested here even though DisableNetworking is
	// true on this runtime instance — resolveNetworkMode only consults
	// DisableNetworking for the NetworkModeDefault case; an explicit
	// NetworkModeBridge request always resolves to bridge regardless.
	if got := egressDisabled.resolveNetworkMode(NetworkModeBridge); got != "bridge" {
		t.Errorf("explicit bridge: expected bridge, got %q", got)
	}
	if got := egressEnabled.resolveNetworkMode(NetworkModeNone); got != NetworkModeNone {
		t.Errorf("explicit none: expected none, got %q", got)
	}
}
