# Session Context

## User Prompts

### Prompt 1

In /Users/jaronswab/go/src/github.com/jrswab/axe/internal/tool/url_fetch.go, modify the `urlFetchExecute` function to integrate HTML stripping. Also add the `"mime"` import.

First, read the current file to see its exact state.

The changes needed are:

1. **Add `"mime"` to the import block** (between `"io"` and `"net/http"` alphabetically)

2. **In `urlFetchExecute`, after line 150 (`bodyStr := string(body)`) and BEFORE the truncation check**, add Content-Type detection and conditional strip...

