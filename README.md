# axe

A CLI tool for managing and running LLM-powered agents.

## Overview

Axe orchestrates LLM-powered agents defined via TOML configuration files. Each
agent has its own system prompt, model selection, skill files, context files,
working directory, persistent memory, and the ability to delegate to sub-agents.

Axe is the executor, not the scheduler. It is designed to be composed with
standard Unix tools — cron, git hooks, pipes, file watchers — rather than
reinventing scheduling or workflow orchestration.

## Features

- **Multi-provider support** — Anthropic, OpenAI, and Ollama (local models)
- **TOML-based agent configuration** — declarative, version-controllable agent definitions
- **Sub-agent delegation** — agents can call other agents via LLM tool use, with depth limiting and parallel execution
- **Persistent memory** — timestamped markdown logs that carry context across runs
- **Memory garbage collection** — LLM-assisted pattern analysis and trimming
- **Skill system** — reusable instruction sets that can be shared across agents
- **Stdin piping** — pipe any output directly into an agent (`git diff | axe run reviewer`)
- **Dry-run mode** — inspect resolved context without calling the LLM
- **JSON output** — structured output with metadata for scripting
- **Minimal dependencies** — two direct dependencies (cobra, toml); all LLM calls use the standard library

## Installation

Requires Go 1.24+.

```bash
go install github.com/jrswab/axe@latest
```

Or build from source:

```bash
git clone https://github.com/jrswab/axe.git
cd axe
go build .
```

## Quick Start

Initialize the configuration directory:

```bash
axe config init
```

This creates the directory structure at `$XDG_CONFIG_HOME/axe/` with a sample
skill and a default `config.toml` for provider credentials.

Scaffold a new agent:

```bash
axe agents init my-agent
```

Edit its configuration:

```bash
axe agents edit my-agent
```

Run the agent:

```bash
axe run my-agent
```

Pipe input from other tools:

```bash
git diff --cached | axe run pr-reviewer
cat error.log | axe run log-analyzer
```

## CLI Reference

### Commands

| Command | Description |
|---|---|
| `axe run <agent>` | Run an agent |
| `axe agents list` | List all configured agents |
| `axe agents show <agent>` | Display an agent's full configuration |
| `axe agents init <agent>` | Scaffold a new agent TOML file |
| `axe agents edit <agent>` | Open an agent TOML in `$EDITOR` |
| `axe config path` | Print the configuration directory path |
| `axe config init` | Initialize the config directory with defaults |
| `axe gc <agent>` | Run memory garbage collection for an agent |
| `axe gc --all` | Run GC on all memory-enabled agents |
| `axe version` | Print the current version |

### Run Flags

| Flag | Default | Description |
|---|---|---|
| `--model <provider/model>` | from TOML | Override the model (e.g. `anthropic/claude-sonnet-4-20250514`) |
| `--skill <path>` | from TOML | Override the skill file path |
| `--workdir <path>` | from TOML or cwd | Override the working directory |
| `--timeout <seconds>` | 120 | Request timeout |
| `--dry-run` | false | Show resolved context without calling the LLM |
| `--verbose` / `-v` | false | Print debug info (model, timing, tokens) to stderr |
| `--json` | false | Wrap output in a JSON envelope with metadata |

## Agent Configuration

Agents are defined as TOML files in `$XDG_CONFIG_HOME/axe/agents/`.

```toml
name = "pr-reviewer"
description = "Reviews pull requests for issues and improvements"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are a senior code reviewer. Be concise and actionable."
skill = "skills/code-review/SKILL.md"
files = ["src/**/*.go", "CONTRIBUTING.md"]
workdir = "/home/user/projects/myapp"
sub_agents = ["test-runner", "lint-checker"]

[sub_agents_config]
max_depth = 3       # maximum nesting depth (hard max: 5)
parallel = true     # run sub-agents concurrently
timeout = 120       # per sub-agent timeout in seconds

[memory]
enabled = true
last_n = 10         # load last N entries into context
max_entries = 100   # warn when exceeded

[params]
temperature = 0.3
max_tokens = 4096
```

All fields except `name` and `model` are optional.

## Providers

| Provider | API Key Env Var | Default Base URL |
|---|---|---|
| Anthropic | `ANTHROPIC_API_KEY` | `https://api.anthropic.com` |
| OpenAI | `OPENAI_API_KEY` | `https://api.openai.com` |
| Ollama | (none required) | `http://localhost:11434` |

Base URLs can be overridden with `AXE_<PROVIDER>_BASE_URL` environment variables
or in `config.toml`.

## License

Apache-2.0 — see [LICENSE](LICENSE).
