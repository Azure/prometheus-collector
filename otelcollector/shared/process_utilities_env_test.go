//go:build linux

package shared

import (
	"os"
	"strings"
	"testing"
)

// TestSetEnvAndSourceBashrcOrPowershell_ValidInput verifies normal settings still reach
// the process environment, which is how child processes receive them
// (cmd.Env = append(os.Environ())).
func TestSetEnvAndSourceBashrcOrPowershell_ValidInput(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"simple", "AZMON_TEST_SIMPLE", "myaccount"},
		{"empty value", "AZMON_TEST_EMPTY", ""},
		{"underscore prefix", "_AZMON_TEST_UNDERSCORE", "value"},
		{"regex value", "AZMON_TEST_REGEX", "kube.*|node_.+"},
		{"value with equals", "AZMON_TEST_EQUALS", "a=b"},
		{"value with spaces", "AZMON_TEST_SPACES", "some value"},
		{"shell metacharacters are stored verbatim", "AZMON_TEST_METACHARS", "a$b`c;d|e&f"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer os.Unsetenv(tc.key)

			if err := SetEnvAndSourceBashrcOrPowershell(tc.key, tc.value, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := os.Getenv(tc.key); got != tc.value {
				t.Errorf("os.Getenv(%q) = %q, want %q", tc.key, got, tc.value)
			}
		})
	}
}

// TestSetEnvAndSourceBashrcOrPowershell_RejectsInvalidKeys ensures malformed names are not
// set. The "export " case is the shape the old prom-config-validator file produced.
func TestSetEnvAndSourceBashrcOrPowershell_RejectsInvalidKeys(t *testing.T) {
	keys := []string{
		"",
		"export AZMON_TEST",
		"AZMON TEST",
		"1AZMON_TEST",
		"AZMON-TEST",
		"AZMON.TEST",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			if err := SetEnvAndSourceBashrcOrPowershell(key, "value", false); err == nil {
				t.Errorf("expected error for key %q, got nil", key)
			}
			if key != "" && os.Getenv(key) != "" {
				t.Errorf("key %q should not have been set", key)
			}
		})
	}
}

// TestSetEnvAndSourceBashrcOrPowershell_RejectsControlCharacters covers values that would
// otherwise add extra lines to the configmapparser env files, which are parsed line by
// line and would therefore be read back as additional key=value pairs.
func TestSetEnvAndSourceBashrcOrPowershell_RejectsControlCharacters(t *testing.T) {
	values := []string{
		"value\nAZMON_INJECTED=true",
		"value\rAZMON_INJECTED=true",
		"value\x00trailing",
	}

	for _, value := range values {
		t.Run(strings.ReplaceAll(value, "\x00", "NUL"), func(t *testing.T) {
			key := "AZMON_TEST_CONTROL"
			defer os.Unsetenv(key)

			if err := SetEnvAndSourceBashrcOrPowershell(key, value, false); err == nil {
				t.Error("expected error for value containing control characters, got nil")
			}
			if os.Getenv(key) != "" {
				t.Error("value with control characters should not have been set")
			}
			if os.Getenv("AZMON_INJECTED") != "" {
				t.Error("a second variable must never be created from one value")
			}
		})
	}
}

// TestSetEnvAndSourceBashrcOrPowershell_NoBashrcWritten asserts the function no longer
// writes to ~/.bashrc. The file was previously used to pass values between the shell
// scripts this agent was ported from; it is no longer read by anything.
func TestSetEnvAndSourceBashrcOrPowershell_NoBashrcWritten(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	key := "AZMON_TEST_NO_BASHRC"
	defer os.Unsetenv(key)

	if err := SetEnvAndSourceBashrcOrPowershell(key, "value", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(home + "/.bashrc"); !os.IsNotExist(err) {
		t.Error("expected ~/.bashrc not to be created")
	}
}
