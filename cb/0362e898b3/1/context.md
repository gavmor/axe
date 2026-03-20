# Session Context

## User Prompts

### Prompt 1

In the axe project at /Users/jaronswab/go/src/github.com/jrswab/axe, fix the nested fenced code block issue in `.opencode/skills/release/SKILL.md`.

The problem is in step 12 where there's a template showing what the release notes should look like. The outer code fence and inner code fence both use triple backticks, which breaks CommonMark parsing.

Find the section that looks like this (around lines 77-85):
```
      ```
      ## Docker

      ```bash
      docker pull ghcr.io/jrswab/axe:X.Y...

