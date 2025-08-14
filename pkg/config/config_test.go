package config

import (
	"testing"
)

func TestGetDefaultCommand(t *testing.T) {
	tests := []struct {
		name           string
		defaultCommand string
		expected       []string
	}{
		{
			name:           "empty command",
			defaultCommand: "",
			expected:       nil,
		},
		{
			name:           "simple command",
			defaultCommand: "bash",
			expected:       []string{"bash"},
		},
		{
			name:           "command with args",
			defaultCommand: "npm run dev",
			expected:       []string{"npm", "run", "dev"},
		},
		{
			name:           "command with quoted string argument",
			defaultCommand: `run_thing "with a string arg"`,
			expected:       []string{"run_thing", "with a string arg"},
		},
		{
			name:           "command with single quotes",
			defaultCommand: `echo 'hello world'`,
			expected:       []string{"echo", "hello world"},
		},
		{
			name:           "command with multiple quoted arguments",
			defaultCommand: `script "first arg" "second arg" third`,
			expected:       []string{"script", "first arg", "second arg", "third"},
		},
		{
			name:           "command with escaped quotes",
			defaultCommand: `echo "hello \"world\""`,
			expected:       []string{"echo", `hello "world"`},
		},
		{
			name:           "complex command with flags and quoted args",
			defaultCommand: `docker run -it --name "my container" ubuntu:latest`,
			expected:       []string{"docker", "run", "-it", "--name", "my container", "ubuntu:latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				DefaultCommand: tt.defaultCommand,
			}
			
			result := cfg.GetDefaultCommand()
			
			if len(result) != len(tt.expected) {
				t.Errorf("GetDefaultCommand() returned %d args, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}
			
			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("GetDefaultCommand() arg[%d] = %q, expected %q", i, arg, tt.expected[i])
				}
			}
		})
	}
}