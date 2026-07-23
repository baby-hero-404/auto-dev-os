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
		{name: "blocked rm", command: []string{"bash", "-lc", "rm -rf /workspace"}, wantErr: true},
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
		{name: "bridge without secrets", req: CommandRequest{NetworkMode: NetworkModeBridge}, resolved: "host", wantErr: false},
		{name: "bridge with secrets", req: CommandRequest{NetworkMode: NetworkModeBridge, SecretEnv: secrets}, resolved: "host", wantErr: true},
		{name: "unsupported network", req: CommandRequest{NetworkMode: "host"}, resolved: "host", wantErr: true},
		// The mismatch the policy previously missed: NetworkModeDefault
		// requested, but the runtime resolves it to an egress-enabled mode
		// because DisableNetworking is false. Secrets must be blocked.
		{name: "default resolved to host with secrets", req: CommandRequest{NetworkMode: NetworkModeDefault, SecretEnv: secrets}, resolved: "host", wantErr: true},
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

	if got := egressEnabled.resolveNetworkMode(NetworkModeDefault); got != "host" {
		t.Errorf("default with egress enabled: expected host, got %q", got)
	}
	if got := egressDisabled.resolveNetworkMode(NetworkModeDefault); got != NetworkModeNone {
		t.Errorf("default with egress disabled: expected none, got %q", got)
	}
	if got := egressDisabled.resolveNetworkMode(NetworkModeBridge); got != "host" {
		t.Errorf("explicit bridge: expected host, got %q", got)
	}
	if got := egressEnabled.resolveNetworkMode(NetworkModeNone); got != NetworkModeNone {
		t.Errorf("explicit none: expected none, got %q", got)
	}
}
