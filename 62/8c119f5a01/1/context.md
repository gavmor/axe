# Session Context

## User Prompts

### Prompt 1

Create the file `.github/workflows/smoke-test.yml` in the repo at `/Users/jaronswab/go/src/github.com/jrswab/axe`.

This is a GitHub Actions workflow for smoke-testing the `axe` binary using real LLM provider calls. Here is the complete specification:

---

## Triggers
- `push` to `master`
- `pull_request`

Same triggers as `.github/workflows/go.yml`.

---

## Jobs Overview

5 jobs total:
1. `dry-run` — no API keys needed, free
2. `providers` — tests Anthropic, OpenAI, OpenCode connectivity
3...

