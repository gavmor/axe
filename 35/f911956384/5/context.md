# Session Context

## User Prompts

### Prompt 1

Create the following TOML file in /Users/jaronswab/go/src/github.com/jrswab/axe/.github/smoke-agents/. Create the directory if it doesn't exist.

The file must be valid TOML parseable by github.com/BurntSushi/toml. String values must be quoted. No fields beyond those specified.

`pipe-basic.toml`:
```toml
name = "pipe-basic"
model = "opencode/minimax-m2.5"
description = "Smoke test: piped stdin input"
```

Return confirmation that the file was created with its exact contents.

