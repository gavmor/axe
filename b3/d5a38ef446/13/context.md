# Session Context

## User Prompts

### Prompt 1

In /Users/jaronswab/go/src/github.com/jrswab/axe/internal/tool/url_fetch.go, replace the current stub `stripHTML` function with the real implementation.

First, read the current file to see its exact state.

The `stripHTML` function must:
1. Parse the HTML string into a DOM tree using `golang.org/x/net/html` (`html.Parse(strings.NewReader(raw))`)
2. If `html.Parse` returns an error, return `raw` unchanged (fallback)
3. Recursively walk the DOM tree
4. Skip `<script>` and `<style>` element nod...

