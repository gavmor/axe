# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Write integration tests for the allowlist feature in `cmd/run_integration_test.go`. Use red/green TDD.

## Context
- Integration tests are in `package cmd` and call `rootCmd.Execute()` in-process.
- They use `testutil.NewMockLLMServer()` to mock the LLM provider.
- They use `writeAgentConfig()` to create temp agent TOML files.
- They use `testutil.SetupXDGDirs()` for isolated XDG...

