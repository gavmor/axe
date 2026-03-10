# Session Context

## User Prompts

### Prompt 1

In /Users/jaronswab/go/src/github.com/jrswab/axe/internal/tool/url_fetch_test.go, find the test `TestURLFetch_HTMLStrippedBeforeTruncation`. 

The current test builds HTML like:
```go
bigCSS := strings.Repeat("x", 9000)
htmlBody := "<html><body><style>" + bigCSS + "</style><p>Short text</p></body></html>"
```

The problem is that the total raw HTML is only ~9050 bytes, which is under maxReadBytes (10000). The test needs the raw HTML to EXCEED maxReadBytes so that WITHOUT stripping it would be...

