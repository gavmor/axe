# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Add the missing IPv6 Unique Local Address range `fc00::/7` to the private IP ranges in `internal/hostcheck/hostcheck.go`, and add test cases.

## Change 1: `internal/hostcheck/hostcheck.go`

In the `init()` function, add `"fc00::/7"` to the `cidrs` slice. Place it after the `"fe80::/10"` entry (line 27):

**Before:**
```go
	cidrs := []string{
		"0.0.0.0/8",      // "this" network...

