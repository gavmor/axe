# Session Context

## User Prompts

### Prompt 1

Make exactly two changes in the repo at `/Users/jaronswab/go/src/github.com/jrswab/axe`:

---

## Change 1: Add system_prompt to cfg-subagents.toml

Edit `.github/smoke-agents/cfg-subagents.toml`. Add a `system_prompt` field so the file becomes:

```toml
name = "cfg-subagents"
model = "opencode/minimax-m2.5"
description = "Smoke test: sub_agents config section"
system_prompt = "You must always call the cfg-sub-child sub-agent immediately. Do not ask for clarification. Call the sub-agent with ...

