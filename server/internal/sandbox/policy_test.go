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
	tests := []struct {
		name    string
		req     CommandRequest
		wantErr bool
	}{
		{name: "default network", req: CommandRequest{}, wantErr: false},
		{name: "bridge without secrets", req: CommandRequest{NetworkMode: NetworkModeBridge}, wantErr: false},
		{name: "bridge with secrets", req: CommandRequest{NetworkMode: NetworkModeBridge, SecretEnv: map[string]string{"TOKEN": "secret"}}, wantErr: true},
		{name: "unsupported network", req: CommandRequest{NetworkMode: "host"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExecutionPolicy(tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateExecutionPolicy() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
