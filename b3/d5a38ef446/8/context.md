# Session Context

## User Prompts

### Prompt 1

In /Users/jaronswab/go/src/github.com/jrswab/axe, investigate the `TestURLFetch_MissingContentTypeNotStripped` test.

The concern is that Go's `net/http` server may auto-detect Content-Type even when we set it to empty string.

Run this test with verbose output:
```bash
cd /Users/jaronswab/go/src/github.com/jrswab/axe && go test ./internal/tool/ -run TestURLFetch_MissingContentTypeNotStripped -v -count=1
```

Also, temporarily add a debug test that checks what Content-Type the server actually...

