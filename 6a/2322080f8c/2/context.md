# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe

Make the following file edits (do NOT commit or push — just edit the files):

1. **cmd/root.go line 12**: Change `const Version = "1.0.0"` to `const Version = "1.2.0"`

2. **cmd/version_test.go**:
   - Line 19: Change `want := "axe version 1.0.0\n"` to `want := "axe version 1.2.0\n"`
   - Line 26: Change `if Version != "1.0.0" {` to `if Version != "1.2.0" {`
   - Line 27: Change `t.Errorf("Version = %q, want %q", Version, "1.0....

