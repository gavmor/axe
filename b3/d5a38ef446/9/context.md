# Session Context

## User Prompts

### Prompt 1

In /Users/jaronswab/go/src/github.com/jrswab/axe, run this Go test to check if `<title>` text from `<head>` leaks into stripHTML output:

```bash
cd /Users/jaronswab/go/src/github.com/jrswab/axe && go test ./internal/tool/ -run TestStripHTML_BasicExtraction -v -count=1
```

Also, write and run a quick ad-hoc test. Create a temporary test file or just use `go test -run` to check: what does `stripHTML("<html><head><title>Page Title</title></head><body><p>Body text</p></body></html>")` return?

...

