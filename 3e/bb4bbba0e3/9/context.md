# Session Context

## User Prompts

### Prompt 1

Verify each finding against the current code and only fix it if needed.

Inline comments:
In `@cmd/run.go`:
- Around line 298-316: The MCP client is left open when Register succeeds but
returns zero routed tools (filtered is empty); after calling
mcpRouter.Register(client, mcpTools, builtinNames) and before appending filtered
to req.Tools, detect if len(filtered) == 0 and close the client (call
client.Close()) to avoid the leak—do this in the same block where you currently
handle verbose logg...

### Prompt 2

go fo rit

