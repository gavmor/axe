# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
The `TestIntegration_AllowedHosts_EmptyAllowlistPermitsPublicHosts` test takes 15 seconds because it makes a real TCP connection attempt to `192.0.2.1` which times out. Fix this by using a URL that will fail faster.

## Better approach
Instead of testing that a public URL gets a connection error (which requires waiting for a timeout), test that the tool result does NOT contain "n...

