# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Fix two issues in `internal/hostcheck/hostcheck.go` using red/green TDD.

### Fix 1a: Nil resolver guard

Before line 84 (`addrs, err := resolver.LookupIPAddr(ctx, hostname)`), add a nil check:

```go
	if resolver == nil {
		return nil, fmt.Errorf("no DNS resolver configured for host %q", hostname)
	}
```

### Fix 1b: Wrap DNS error with hostname context

Change line 85-86 from:
...

