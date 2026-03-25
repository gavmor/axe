# Session Context

## User Prompts

### Prompt 1

In the repo at `/Users/jaronswab/go/src/github.com/jrswab/axe` on branch `ISS-40-cicd-workflow`:

1. Run `git diff --stat` to confirm only `.github/workflows/smoke-test.yml` is modified.
2. Stage and commit:
```
git add .github/workflows/smoke-test.yml
git commit -m "fix: guard secret jobs on fork PRs, fix grep --"
```
3. Push:
```
git push
```

Report the output of each command.

