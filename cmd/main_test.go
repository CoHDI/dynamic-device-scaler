/*
Copyright 2025 The CoHDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"strings"
	"testing"
)

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name        string
		setEnv      bool
		envValue    string
		defaultVal  int
		wantValue   int
		wantErr     bool
		errContains string
	}{
		{
			name:       "Valid environment value",
			setEnv:     true,
			envValue:   "300",
			defaultVal: 500,
			wantValue:  300,
			wantErr:    false,
		},
		{
			name:        "Invalid environment value (non-integer)",
			setEnv:      true,
			envValue:    "abc",
			defaultVal:  500,
			wantValue:   0,
			wantErr:     true,
			errContains: "invalid integer value",
		},
		{
			name:       "Empty environment value",
			setEnv:     true,
			envValue:   "",
			defaultVal: 500,
			wantValue:  500,
			wantErr:    false,
		},
		{
			name:       "Environment variable not set",
			setEnv:     false,
			defaultVal: 500,
			wantValue:  500,
			wantErr:    false,
		},
		{
			name:        "Negative value (out of range)",
			setEnv:      true,
			envValue:    "-100",
			defaultVal:  500,
			wantValue:   0,
			wantErr:     true,
			errContains: "must be between 0-86400",
		},
		{
			name:        "Value exceeds 86400",
			setEnv:      true,
			envValue:    "86401",
			defaultVal:  500,
			wantValue:   0,
			wantErr:     true,
			errContains: "must be between 0-86400",
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

			gotValue, err := getEnvAsInt(envVarName, tt.defaultVal)

			if gotValue != tt.wantValue {
				t.Errorf("value mismatch: want %d, got %d (env=%q, default=%d)",
					tt.wantValue, gotValue, tt.envValue, tt.defaultVal)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("error presence mismatch: wantErr=%t, gotErr=%v (env=%q)",
					tt.wantErr, err, tt.envValue)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error message check failed: want contains %q, got %q",
						tt.errContains, err.Error())
				}
			}
		})
	}
}
