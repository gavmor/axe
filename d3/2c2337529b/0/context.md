# Session Context

## User Prompts

### Prompt 1

In the repo at `/Users/jaronswab/go/src/github.com/jrswab/axe`, amend the commit message. Run:

```
git commit --amend -m "feat: add smoke-test CI workflow

Add .github/workflows/smoke-test.yml with five parallel jobs
that exercise the axe binary against real LLM providers. The
dry-run job validates context resolution without API keys. The
providers, tools, config-sections, and piped-input jobs verify
Anthropic, OpenAI, and OpenCode connectivity, all seven built-in
tools, TOML config parsing,...

