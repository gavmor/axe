# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Fix 3 issues in the integration tests in `cmd/run_integration_test.go`:

### Fix 1: `TestIntegration_AllowedHosts_EmptyAllowlistPermitsPublicHosts`

This test makes a real DNS query to `nonexistent.example.com` which is flaky in air-gapped CI. Change the URL to use `https://192.0.2.1/data` (TEST-NET-1 from RFC 5737 — a public IP that's guaranteed to not be routable but is NOT in ...

