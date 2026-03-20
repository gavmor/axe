# Session Context

## User Prompts

### Prompt 1

You are refactoring tests in `/Users/jaronswab/go/src/github.com/jrswab/axe/internal/tool/url_fetch_test.go`.

## Context

Lines 688-843 contain 6 separate test functions for allowlist and IP-check behavior. They should be combined into a single table-driven test. The file is in package `tool` (internal test).

## What to do (TDD approach)

### Step 1: Write the new table-driven test, then delete the old tests

Replace everything from line 688 (`// Phase 4: Allowlist and private IP tests`) th...

