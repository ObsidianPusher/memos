package main

import (
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

func loadTestConfig(t *testing.T, content string) string {
	t.Helper()
	oldFile, oldDir := cfgFile, activeConfigDir
	t.Cleanup(func() { cfgFile, activeConfigDir = oldFile, oldDir; viper.Set("data", nil) })
	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "memos.yaml")
	if err := os.WriteFile(cfgFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := initConfig(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestConfigUnderscoreKeys(t *testing.T) {
	loadTestConfig(t, "unix_sock: /tmp/memos-test.sock\ninstance_url: https://memos.example\n")
	if got := viper.GetString("unix-sock"); got != "/tmp/memos-test.sock" {
		t.Fatalf("unix-sock = %q", got)
	}
	if got := viper.GetString("instance-url"); got != "https://memos.example" {
		t.Fatalf("instance-url = %q", got)
	}
}

func TestConfigRelativeDataUsesConfigDirectory(t *testing.T) {
	dir := loadTestConfig(t, "data: ./notes\n")
	if got := viper.GetString("data"); got != filepath.Join(dir, "notes") {
		t.Fatalf("data = %q, want config-relative path", got)
	}
}

func TestConfigEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("MEMOS_DATA", t.TempDir())
	t.Setenv("MEMOS_INSTANCE_URL", "https://override.example")
	t.Setenv("MEMOS_UNIX_SOCK", "/tmp/override.sock")
	loadTestConfig(t, "data: ./notes\ninstance_url: https://yaml.example\nunix_sock: /tmp/yaml.sock\n")
	if got := viper.GetString("data"); got != os.Getenv("MEMOS_DATA") {
		t.Fatalf("data override = %q", got)
	}
	if got := viper.GetString("unix-sock"); got != "/tmp/override.sock" {
		t.Fatalf("socket override = %q", got)
	}
	if got := viper.GetString("instance-url"); got != "https://override.example" {
		t.Fatalf("url override = %q", got)
	}
}

func TestConfigFlagsOverrideEnvironment(t *testing.T) {
	t.Setenv("MEMOS_DATA", t.TempDir())
	t.Setenv("MEMOS_UNIX_SOCK", "/tmp/env.sock")
	flags := rootCmd.PersistentFlags()
	for name, value := range map[string]string{"data": "./flag-data", "unix-sock": "/tmp/flag.sock"} {
		flag := flags.Lookup(name)
		oldValue, oldChanged := flag.Value.String(), flag.Changed
		t.Cleanup(func() { _ = flag.Value.Set(oldValue); flag.Changed = oldChanged })
		if err := flags.Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	dir := loadTestConfig(t, "data: ./yaml-data\nunix_sock: /tmp/yaml.sock\n")
	if got := viper.GetString("data"); got != filepath.Join(dir, "flag-data") {
		t.Fatalf("flag data = %q", got)
	}
	if got := viper.GetString("unix-sock"); got != "/tmp/flag.sock" {
		t.Fatalf("flag socket = %q", got)
	}
}

func TestConfigCanonicalKeysOverrideLegacyKeys(t *testing.T) {
	loadTestConfig(t, "unix_sock: /tmp/legacy.sock\nunix-sock: /tmp/canonical.sock\ninstance_url: https://legacy.example\ninstance-url: https://canonical.example\n")
	if got := viper.GetString("unix-sock"); got != "/tmp/canonical.sock" {
		t.Fatalf("canonical socket = %q", got)
	}
	if got := viper.GetString("instance-url"); got != "https://canonical.example" {
		t.Fatalf("canonical URL = %q", got)
	}
}
