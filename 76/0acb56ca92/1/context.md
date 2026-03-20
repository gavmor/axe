# Session Context

## User Prompts

### Prompt 1

In the axe project at /Users/jaronswab/go/src/github.com/jrswab/axe, run the unit tests to verify the version bump and ensure nothing is broken.

Run these commands and return the output:

1. `cd /Users/jaronswab/go/src/github.com/jrswab/axe && go test ./cmd/ -run "TestVersion" -count=1 -v`
2. `cd /Users/jaronswab/go/src/github.com/jrswab/axe && go test ./internal/mcpclient/ -count=1 -v -short 2>&1 | head -50`
3. `cd /Users/jaronswab/go/src/github.com/jrswab/axe && go build .`

Return the ful...

