# Session Context

## User Prompts

### Prompt 1

You are in the repo at /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-36/docker-build-in-goreleaser.

Edit the file `.github/workflows/release.yml` to narrow the permission scope. Here is the exact change needed:

**Current top-level permissions block (lines 10-14):**
```yaml
permissions:
  contents: write      # GoReleaser: create GitHub Release
  packages: write      # Docker: push to GHCR
  attestations: write  # Docker: build provenance attestation
  id-token: write      # Do...

