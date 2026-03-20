# Session Context

## User Prompts

### Prompt 1

You are in the repo at /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-36/docker-build-in-goreleaser.

Run the following commands in order:

1. `git add .github/workflows/release.yml`
2. `git commit -m "ci: narrow workflow permissions and gate docker behind goreleaser

- Move packages, attestations, and id-token write permissions from
  workflow-level to docker job-level (least privilege)
- Add needs: [goreleaser] to docker job so it only runs after
  the GitHub Release is success...

