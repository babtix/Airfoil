package config

import "testing"

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"simple", "FOO=bar", "FOO", "bar", true},
		{"spaces around equals", "FOO = bar", "FOO", "bar", true},
		{"export prefix", "export FOO=bar", "FOO", "bar", true},
		{"empty value", "FOO=", "FOO", "", true},
		{"double quoted", `FOO="bar baz"`, "FOO", "bar baz", true},
		{"single quoted", `FOO='bar baz'`, "FOO", "bar baz", true},
		{"quotes preserve trailing hash", `FOO="bar # not a comment"`, "FOO", "bar # not a comment", true},
		{"unquoted trailing comment", "FOO=bar # comment", "FOO", "bar", true},
		{"hash without space is part of value", "FOO=bar#baz", "FOO", "bar#baz", true},
		{"value contains equals", "FOO=a=b", "FOO", "a=b", true},
		{"url value", "NVIDIA_NIM_BASE_URL=https://integrate.api.nvidia.com/v1", "NVIDIA_NIM_BASE_URL", "https://integrate.api.nvidia.com/v1", true},
		{"reddit ua with slashes", "UA=airfoil/0.1 (by /u/someone)", "UA", "airfoil/0.1 (by /u/someone)", true},

		{"blank", "", "", "", false},
		{"whitespace only", "   ", "", "", false},
		{"comment", "# a comment", "", "", false},
		{"indented comment", "   # a comment", "", "", false},

		{"no equals is malformed", "JUST_A_NAME", "", "", true},
		{"empty key is malformed", "=value", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val, ok := parseDotEnvLine(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
			if val != tt.wantVal {
				t.Errorf("val = %q, want %q", val, tt.wantVal)
			}
		})
	}
}
