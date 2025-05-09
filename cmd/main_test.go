package main

import (
	"os"
	"testing"
)

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name       string
		setEnv     bool
		envValue   string
		defaultVal int
		expected   int
	}{
		{
			name:       "Valid environment value",
			setEnv:     true,
			envValue:   "300",
			defaultVal: 500,
			expected:   300,
		},
		{
			name:       "Invalid environment value (non-integer)",
			setEnv:     true,
			envValue:   "abc",
			defaultVal: 500,
			expected:   500,
		},
		{
			name:       "Empty environment value",
			setEnv:     true,
			envValue:   "",
			defaultVal: 500,
			expected:   500,
		},
		{
			name:       "Environment variable not set",
			setEnv:     false,
			defaultVal: 500,
			expected:   500,
		},
		{
			name:       "Zero value",
			setEnv:     true,
			envValue:   "0",
			defaultVal: 500,
			expected:   0,
		},
		{
			name:       "Negative value",
			setEnv:     true,
			envValue:   "-100",
			defaultVal: 500,
			expected:   -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVarName := "TEST_ENV_VAR"

			if tt.setEnv {
				t.Setenv(envVarName, tt.envValue)
			} else {
				os.Unsetenv(envVarName)
			}

			actual := getEnvAsInt(envVarName, tt.defaultVal)

			if actual != tt.expected {
				t.Errorf("expected %d, got %d (env=%q, default=%d)",
					tt.expected, actual, tt.envValue, tt.defaultVal)
			}
		})
	}
}
