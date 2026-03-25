# Session Context

## User Prompts

### Prompt 1

In the file `/Users/jaronswab/go/src/github.com/jrswab/axe/.github/workflows/smoke-test.yml`, make exactly one change:

On line 54, change:
```
        echo "$OUTPUT" | grep -q -- "--- Files ---" || { echo "FAIL: '--- Files ---' not found"; exit 1; }
```
to:
```
        echo "$OUTPUT" | grep -q -- "--- Files" || { echo "FAIL: '--- Files' not found"; exit 1; }
```

The actual dry-run output is `--- Files (1) ---` (with a count), so the pattern must match the invariant prefix `--- Files` rather...

