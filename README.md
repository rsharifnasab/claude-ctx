# cluade-ctx

A command-line utility for managing multiple login accounts/contexts for Claude Code.

## Overview

`cluade-ctx` allows you to manage multiple Claude API configurations (accounts) with different environment variables. This is useful if you work with multiple Claude API endpoints, authentication tokens, or model preferences.

## Features

- **Multiple Accounts**: Create and manage multiple named accounts with different configurations
- **Quick Switching**: Switch between accounts with a single command
- **Interactive Selection**: Use the interactive TUI to browse and select accounts
- **Per-Project Settings**: Configuration is stored in the current working directory

## Installation

```bash
# Build from source
go build -o bin/cluade-ctx main.go

# Or install globally
go install
```

Make sure the binary is in your PATH.

## Usage

```
Usage:
  cluade-ctx current           # Show current account
  cluade-ctx switch <name>    # Switch to an account
  cluade-ctx accounts         # Interactive account selector
  cluade-ctx add-account <name> [KEY=VALUE ...]  # Add a new account
  cluade-ctx remove <name>     # Remove an account
```

### Commands

#### `cluade-ctx current`

Display the currently active account.

```bash
$ cluade-ctx current
dev
```

#### `cluade-ctx switch <name>`

Switch to a different account. This updates the `settings.json` file with the environment variables for that account.

```bash
$ cluade-ctx switch prod
Switched to account "prod"
```

#### `cluade-ctx accounts`

Open an interactive TUI to select an account. Use arrow keys (or `j`/`k`) to navigate, `Enter` to select, and `q` to quit.

```bash
$ cluade-ctx accounts
```

#### `cluade-ctx add-account <name> [KEY=VALUE ...]`

Add a new account with environment variables.

```bash
# Add account with just a name
cluade-ctx add-account personal

# Add account with environment variables
cluade-ctx add-account dev ANTHROPIC_AUTH_TOKEN=sk-xxx ANTHROPIC_BASE_URL=https://api.anthropic.com
```

#### `cluade-ctx remove <name>`

Remove an account. You cannot remove the currently active account.

```bash
$ cluade-ctx remove old-account
Removed account "old-account"
```

## Configuration

### Directory Structure

The tool creates/uses the following files in the current working directory:

- `config.yaml` - Stores account configurations
- `settings.json` - Claude Code settings (updated when switching accounts)

### config.yaml Format

```yaml
accounts:
  - name: "disabled"
    env: {}
  - name: "dev"
    env:
      "ANTHROPIC_AUTH_TOKEN": "sk-xxx"
      "ANTHROPIC_BASE_URL": "https://api.anthropic.com"
  - name: "prod"
    env:
      "ANTHROPIC_AUTH_TOKEN": "sk-yyy"
      "ANTHROPIC_BASE_URL": "https://api.anthropic.com"
current-account: "dev"
```

### Environment Variables

Common environment variables for Claude Code:

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_AUTH_TOKEN` | Authentication token for Claude API |
| `ANTHROPIC_BASE_URL` | Custom API endpoint URL |
| `ANTHROPIC_DEFAULT_MODEL` | Default model to use |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Default Haiku model |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Default Sonnet model |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Default Opus model |
| `API_TIMEOUT_MS` | API request timeout in milliseconds |

## Example Workflow

```bash
# Add a development account
cluade-ctx add-account dev ANTHROPIC_AUTH_TOKEN=sk-dev-xxx ANTHROPIC_BASE_URL=https://dev-api.example.com

# Add a production account
cluade-ctx add-account prod ANTHROPIC_AUTH_TOKEN=sk-prod-yyy ANTHROPIC_BASE_URL=https://api.example.com

# Check current account
cluade-ctx current

# Switch to production
cluade-ctx switch prod

# List all accounts interactively
cluade-ctx accounts
```

## Requirements

- Go 1.26+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) (for interactive TUI)

## License

MIT
