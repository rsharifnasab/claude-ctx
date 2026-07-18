package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var config *Config

func init() {
	baseDir := localBaseDir()

	settings, err := ensureLocalSettings(baseDir)
	if err != nil {
		log.Fatal(err)
	}

	config, err = loadOrCreateConfig(baseDir, settings)
	if err != nil {
		log.Fatalln(err)
	}
}

type Account struct {
	Name string            `yaml:"name"`
	Env  map[string]string `yaml:"env"`
}

type Config struct {
	Accounts       []Account `yaml:"accounts"`
	CurrentAccount string    `yaml:"current-account"`
}

var errNoLocalSettings = errors.New("no local settings available")

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	baseDir := localBaseDir()

	switch os.Args[1] {
	case "current":
		if err := runCurrent(baseDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "switch":
		if err := runSwitch(baseDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "accounts":
		if err := runAccounts(baseDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "add-account":
		if err := runAddAccount(baseDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "remove":
		if err := runRemoveAccount(baseDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  cluade-ctx current \n\t --show: if exists, print the current config")
	fmt.Println("  cluade-ctx switch <name>")
	fmt.Println("  cluade-ctx accounts")
	fmt.Println("  cluade-ctx add-account <name> [KEY=VALUE ...]")
	fmt.Println("  cluade-ctx remove <name>")
}

func localBaseDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func homeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return "", fmt.Errorf("unable to determine home directory")
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func runAddAccount(baseDir string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cluade-ctx add-account <name> [KEY=VALUE ...]")
	}

	name := args[0]

	env := make(map[string]string)
	for _, entry := range args[1:] {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("invalid env entry %q (expected KEY=VALUE)", entry)
		}
		env[parts[0]] = parts[1]
	}

	for _, account := range config.Accounts {
		if account.Name == name {
			return fmt.Errorf("account %q already exists", name)
		}
	}

	config.Accounts = append(config.Accounts, Account{Name: name, Env: env})

	if err := saveConfig(baseDir, config); err != nil {
		return err
	}

	fmt.Printf("Added account %q\n", name)
	return nil
}

func runRemoveAccount(baseDir string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: cluade-ctx remove <name>")
	}

	name := args[0]

	// Check if account exists
	foundIndex := -1
	for i, account := range config.Accounts {
		if account.Name == name {
			foundIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return fmt.Errorf("account %q does not exist", name)
	}

	// Check if account is currently in use
	if config.CurrentAccount == name {
		return fmt.Errorf("account %q is currently in use and cannot be deleted", name)
	}

	// Remove the account
	config.Accounts = append(config.Accounts[:foundIndex], config.Accounts[foundIndex+1:]...)

	if err := saveConfig(baseDir, config); err != nil {
		return err
	}

	fmt.Printf("Removed account %q\n", name)
	return nil
}

func runCurrent(baseDir string, args []string) error {
	show := false
	for _, arg := range args {
		if arg == "--show" {
			show = true
		} else {
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if config.CurrentAccount == "" {
		fmt.Println("No current account configured")
		return nil
	}

	account, ok := config.findAccount(config.CurrentAccount)
	if !ok {
		return fmt.Errorf("current account %q not found", config.CurrentAccount)
	}

	if show {
		fmt.Printf("Name: %s\n", account.Name)
		fmt.Println("Environment:")
		if len(account.Env) > 0 {
			keys := make([]string, 0, len(account.Env))
			for key := range account.Env {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Printf("  %s=%s\n", key, account.Env[key])
			}
		}
		return nil
	}

	fmt.Println(account.Name)
	return nil
}

func runSwitch(baseDir string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: cluade-ctx switch <name>")
	}

	account, ok := config.findAccount(args[0])
	if !ok {
		return fmt.Errorf("account %q not found", args[0])
	}

	config.CurrentAccount = account.Name
	if err := saveConfig(baseDir, config); err != nil {
		return err
	}
	if err := updateSettingsSnapshot(baseDir, account.Env); err != nil {
		return err
	}

	fmt.Printf("Switched to account %q\n", account.Name)
	return nil
}

func loadConfig(baseDir string) (*Config, error) {
	configPath := filepath.Join(baseDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg := &Config{
			Accounts:       []Account{{Name: "disabled", Env: map[string]string{}}},
			CurrentAccount: "disabled",
		}
		if err := saveConfig(baseDir, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	cfg, err := unmarshalConfig(string(data))
	if err != nil {
		return nil, err
	}
	if cfg.Accounts == nil {
		cfg.Accounts = []Account{}
	}
	return cfg, nil
}

func loadOrCreateConfig(baseDir string, settings map[string]any) (*Config, error) {
	configPath := filepath.Join(baseDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		// config.yaml doesn't exist, create one
		cfg := &Config{
			Accounts:       []Account{{Name: "disabled", Env: map[string]string{}}},
			CurrentAccount: "disabled",
		}

		// Check if settings contains "env" key
		if env, ok := settings["env"]; ok {
			envMap, ok := env.(map[string]any)
			if ok {
				// Convert map[string]any to map[string]string
				envStrMap := make(map[string]string)
				for k, v := range envMap {
					if vStr, ok := v.(string); ok {
						envStrMap[k] = vStr
					}
				}
				// Add "default" account with the env
				cfg.Accounts = append(cfg.Accounts, Account{Name: "default", Env: envStrMap})
				cfg.CurrentAccount = "default"
			}
		}

		if err := saveConfig(baseDir, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	cfg, err := unmarshalConfig(string(data))
	if err != nil {
		return nil, err
	}
	if cfg.Accounts == nil {
		cfg.Accounts = []Account{}
	}
	return cfg, nil
}

func saveConfig(baseDir string, cfg *Config) error {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	content, err := marshalConfig(cfg)
	if err != nil {
		return err
	}
	configPath := filepath.Join(baseDir, "config.yaml")
	return os.WriteFile(configPath, content, 0o644)
}

func marshalConfig(cfg *Config) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("accounts:\n")
	for _, account := range cfg.Accounts {
		builder.WriteString(fmt.Sprintf("  - name: %s\n", escapeYAMLValue(account.Name)))
		if len(account.Env) == 0 {
			builder.WriteString("    env: {}\n")
			continue
		}
		builder.WriteString("    env:\n")
		keys := make([]string, 0, len(account.Env))
		for key := range account.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			builder.WriteString(fmt.Sprintf("      %s: %s\n", escapeYAMLValue(key), escapeYAMLValue(account.Env[key])))
		}
	}
	builder.WriteString(fmt.Sprintf("current-account: %s\n", escapeYAMLValue(cfg.CurrentAccount)))
	return []byte(builder.String()), nil
}

func unmarshalConfig(content string) (*Config, error) {
	cfg := &Config{Accounts: []Account{}}
	var currentAccount *Account
	inEnv := false

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "accounts:") {
			inEnv = false
			continue
		}
		if strings.HasPrefix(line, "current-account:") {
			value, err := parseYAMLValue(strings.TrimSpace(strings.TrimPrefix(line, "current-account:")))
			if err != nil {
				return nil, err
			}
			cfg.CurrentAccount = value
			inEnv = false
			continue
		}
		if strings.HasPrefix(line, "- name:") {
			name, err := parseYAMLValue(strings.TrimSpace(strings.TrimPrefix(line, "- name:")))
			if err != nil {
				return nil, err
			}
			cfg.Accounts = append(cfg.Accounts, Account{Name: name, Env: map[string]string{}})
			currentAccount = &cfg.Accounts[len(cfg.Accounts)-1]
			inEnv = false
			continue
		}
		if strings.HasPrefix(line, "env:") {
			if len(strings.TrimSpace(strings.TrimPrefix(line, "env:"))) == 0 {
				inEnv = true
				continue
			}
			inEnv = false
			continue
		}
		if inEnv && currentAccount != nil {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid env entry %q", line)
			}
			key, err := parseYAMLValue(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, err
			}
			value, err := parseYAMLValue(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
			}
			currentAccount.Env[key] = value
		}
	}

	if len(cfg.Accounts) == 0 {
		cfg.Accounts = []Account{}
	}
	return cfg, nil
}

func escapeYAMLValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", value)
}

func parseYAMLValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
		return strings.Trim(value, "\""), nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return strings.Trim(value, "'"), nil
	}
	return value, nil
}

func writeCurrentSettings(baseDir string, cfg *Config) error {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	// Ensure local settings.json exists first
	if _, err := ensureLocalSettings(baseDir); err != nil {
		return err
	}

	account, ok := cfg.findAccount(cfg.CurrentAccount)
	if !ok {
		return fmt.Errorf("current account %q not found", cfg.CurrentAccount)
	}
	return updateSettingsSnapshot(baseDir, account.Env)
}

func ensureLocalSettings(baseDir string) (map[string]any, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}

	localSettingsPath := filepath.Join(baseDir, "settings.json")
	if _, statErr := os.Stat(localSettingsPath); statErr == nil {
		// Local settings.json exists, read and return its content
		content, err := os.ReadFile(localSettingsPath)
		if err != nil {
			return nil, err
		}
		payload := map[string]any{}
		if len(content) > 0 {
			if err := json.Unmarshal(content, &payload); err != nil {
				return nil, err
			}
		}
		return payload, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}

	// Local settings.json does not exist, check home settings
	homeSettingsPath, err := homeSettingsPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(homeSettingsPath); err == nil {
		// Home settings.json exists, copy to local
		content, err := os.ReadFile(homeSettingsPath)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(localSettingsPath, content, 0o644); err != nil {
			return nil, err
		}

		createHardLink(localSettingsPath, homeSettingsPath, true)

		// Parse and return the content
		payload := map[string]any{}
		if len(content) > 0 {
			if err := json.Unmarshal(content, &payload); err != nil {
				return nil, err
			}
		}
		return payload, nil
	}

	// Neither exists, create empty local settings.json
	if err := os.WriteFile(localSettingsPath, []byte("{}\n"), 0o644); err != nil {
		return nil, err
	}

	createHardLink(localSettingsPath, homeSettingsPath, true)

	return map[string]any{}, nil
}

func updateSettingsSnapshot(baseDir string, env map[string]string) error {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	settingsPath := filepath.Join(baseDir, "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("local settings.json does not exist")
		}
		return err
	}

	payload := map[string]any{}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &payload); err != nil {
			return err
		}
	}

	newEnv := map[string]string{}
	for key, value := range env {
		newEnv[key] = value
	}
	payload["env"] = newEnv

	content, err = json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(settingsPath, content, 0o644)
}

func runAccounts(baseDir string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: cluade-ctx accounts")
	}

	selected := 0
	for i, account := range config.Accounts {
		if account.Name == config.CurrentAccount {
			selected = i
			break
		}
	}

	model := accountsModel{config: config, selected: selected}
	p := tea.NewProgram(model)
	final, err := p.Run()
	if err != nil {
		return err
	}

	if result, ok := final.(accountsModel); ok && result.chosen != "" {
		return runSwitch(baseDir, []string{result.chosen})
	}
	return nil
}

type accountsModel struct {
	config   *Config
	selected int
	chosen   string
}

func (m accountsModel) Init() tea.Cmd { return nil }

func (m accountsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.config.Accounts)-1 {
				m.selected++
			}
		case "enter":
			m.chosen = m.config.Accounts[m.selected].Name
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m accountsModel) View() string {
	if len(m.config.Accounts) == 0 {
		return "No accounts configured.\n"
	}

	var b strings.Builder
	b.WriteString("Accounts:\n\n")
	for i, account := range m.config.Accounts {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		b.WriteString(prefix + account.Name)
		if account.Name == m.config.CurrentAccount {
			b.WriteString(" *")
		}
		b.WriteString("\n")
	}
	b.WriteString("\nUse up/down or j/k to move, enter to select, q to quit.\n")
	return b.String()
}

func (cfg *Config) findAccount(name string) (Account, bool) {
	for _, account := range cfg.Accounts {
		if account.Name == name {
			return account, true
		}
	}
	return Account{}, false
}

func createHardLink(src, dest string, force bool) error {
	_, errStat := os.Stat(dest)
	if errStat != nil && !os.IsNotExist(errStat) {
		return errStat
	}
	// file exist, then if force is true remove the dest
	if errStat == nil && force {
		if err := os.Remove(dest); err != nil {
			return err
		}
	}

	return os.Symlink(src, dest)
}
