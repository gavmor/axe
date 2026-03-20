# Session Context

## User Prompts

### Prompt 1

There is a remote branch for this repo called `ISS-36/docker-build-in-goreleaser` we need to work on; pull it down and switch to it

### Prompt 2

Verify each finding against the current code and only fix it if needed.

In @.github/workflows/release.yml around lines 11 - 14, The top-level workflow
permissions currently grant packages, attestations and id-token globally; narrow
scope by removing those keys from the workflow-wide permissions block and
instead add minimal permissions to only the jobs that need them (e.g., the
release/publish job that runs GoReleaser and the Docker/ghcr push job).
Specifically, keep only necessary global pe...

### Prompt 3

Verify each finding against the current code and only fix it if needed.

In @.github/workflows/release.yml around lines 42 - 85, The docker publishing
job currently named "docker" must be gated behind verification and release
success: create or reference a verification job (e.g., "verify" or existing
test/lint jobs) and the release creation job (e.g., "release") and add a needs:
[verify, release] to the docker job so it only runs after tests/lint pass and
the release is created; also ensure t...

### Prompt 4

go ahead with the change

### Prompt 5

commit the changes and push to remote

