# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Revert the export of `URLFetchTimeout` back to `urlFetchTimeout` (unexported). The integration test should not need to override internal package state. Instead, redesign the test to avoid the timeout issue entirely.

## Changes needed

### 1. Revert `internal/tool/url_fetch.go`
Change `URLFetchTimeout` back to `urlFetchTimeout` on line 23 and all references in the file (lines 23,...

