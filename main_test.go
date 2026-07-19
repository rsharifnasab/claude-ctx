package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Accounts:       []Account{{Name: "dev", Env: map[string]string{"API_URL": "https://dev.example", "TOKEN": "abc"}}},
		CurrentAccount: "dev",
	}

	if err := saveConfig(tmpDir, cfg); err != nil {
		t.Fatalf("saveConfig() error = %v", err)
	}

	loaded, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if loaded.CurrentAccount != "dev" {
		t.Fatalf("expected current account dev, got %q", loaded.CurrentAccount)
	}
	if len(loaded.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(loaded.Accounts))
	}
	if loaded.Accounts[0].Env["TOKEN"] != "abc" {
		t.Fatalf("expected env token abc, got %q", loaded.Accounts[0].Env["TOKEN"])
	}
}

func TestWriteCurrentSettingsCreatesSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Accounts:       []Account{{Name: "prod", Env: map[string]string{"API_URL": "https://prod.example"}}},
		CurrentAccount: "prod",
	}

	if err := writeCurrentSettings(tmpDir, cfg); err != nil {
		t.Fatalf("writeCurrentSettings() error = %v", err)
	}

	linkPath := filepath.Join(tmpDir, "settings.json")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat(settings.json) error = %v", err)
	}
	// Check that settings.json exists (either as regular file or symlink)
	if info.Mode().IsRegular() == false && info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected settings.json to be a file or symlink, got mode %v", info.Mode())
	}

	content, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("ReadFile(settings.json) error = %v", err)
	}
	// The env is stored under the "env" key
	if !strings.Contains(string(content), `"env":`) || !strings.Contains(string(content), `"API_URL": "https://prod.example"`) {
		t.Fatalf("unexpected settings content: %s", string(content))
	}
}

func TestEnsureSettingsSnapshotCopiesHomeFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("accounts:\n  - name: default\n    env:\n      FOO: bar\ncurrent-account: default\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	homeSettingsDir := filepath.Join(tempHome, ".claude")
	if err := os.MkdirAll(homeSettingsDir, 0o755); err != nil {
		t.Fatalf("mkdir home settings dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeSettingsDir, "settings.json"), []byte(`{"env":{"A":"1"},"other":"value"}`), 0o644); err != nil {
		t.Fatalf("write home settings: %v", err)
	}

	settings, err := ensureLocalSettings(configDir)
	if err != nil {
		t.Fatalf("ensureLocalSettings() error = %v", err)
	}

	// Verify the settings were loaded correctly
	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected env to be present in settings")
	}
	if env["A"] != "1" {
		t.Fatalf("expected env.A to be '1', got %v", env["A"])
	}
	other, ok := settings["other"].(string)
	if !ok || other != "value" {
		t.Fatalf("expected other to be 'value', got %v", other)
	}
}

func TestRunSwitchUpdatesSettingsEnvOnly(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`accounts:
  - name: default
    env:
      A: one
      B: two
  - name: work
    env:
      A: three
      C: four
current-account: default
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"env":{"A":"old","B":"old"},"other":"preserve"}`), 0o644); err != nil {
		t.Fatalf("write settings snapshot: %v", err)
	}

	if err := runSwitch(configDir, "work"); err != nil {
		t.Fatalf("runSwitch() error = %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(configDir, "settings.json"))
	if err != nil {
		t.Fatalf("read updated settings: %v", err)
	}
	if !strings.Contains(string(updated), `"other": "preserve"`) {
		t.Fatalf("expected other field to be preserved, got %s", string(updated))
	}
	if !strings.Contains(string(updated), `"A": "three"`) || !strings.Contains(string(updated), `"C": "four"`) {
		t.Fatalf("expected env values to be updated, got %s", string(updated))
	}

	cfg, err := loadConfig(configDir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.CurrentAccount != "work" {
		t.Fatalf("expected current account to be work, got %q", cfg.CurrentAccount)
	}
}
