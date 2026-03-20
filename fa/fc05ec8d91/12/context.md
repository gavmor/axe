# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Add `AllowedHosts []string` to the `AgentConfig` struct and write TOML decode tests. Use red/green TDD.

## Step 1: Write tests FIRST — `internal/agent/agent_test.go`

Add the following tests to the EXISTING test file (append, don't overwrite). Follow the existing pattern in the file which uses raw TOML string → `tomlDecode(input, &cfg)` → assert fields.

The `tomlDecode` var is ...

