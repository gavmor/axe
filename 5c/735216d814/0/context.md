# Session Context

## User Prompts

### Prompt 1

# Implementation Instructions

1. Study the spec markdown file thoroughly
    - located in `./docs/plans/`
    - Use the spec file that starts with 025
    - If 025 is empty, blank, or does not exist; the newest file formatted as `xxx_topic_spec.md`
2. Study implementation plan markdown file thoroughly
    - located in `./docs/plans/`
    - Use the spec file that starts with 025
    - If 025 is empty, blank, or does not exist; the newest file formatted as `xxx_topic_implement.md`
3. Pick the ...

### Prompt 2

are there any steps left?

### Prompt 3

commit the changes

### Prompt 4

1. Verify that we are not on the develop branch.
    - If we are on the develop branch ask what branch name to switch to.
    - Once given the name create a new git branch off of develop using `git checkout -b`
2. Review all changes made against develop.
3. Create a Git message which lists out the changes made in a short paragraph
4. The title should summarize the changes at a high level. Keep it UNDER 50 characters.
5. Commit the changes locally.
6. Push the branch to remote using `git push ...

### Prompt 5

rebase this on master

### Prompt 6

commit docs/plans/025_url_fetch_spec.md

### Prompt 7

change the branch name to iss-007/url-fetch-tool

### Prompt 8

1. Verify that we are not on the develop branch.
    - If we are on the develop branch ask what branch name to switch to.
    - Once given the name create a new git branch off of develop using `git checkout -b`
2. Review all changes made against develop.
3. Create a Git message which lists out the changes made in a short paragraph
4. The title should summarize the changes at a high level. Keep it UNDER 50 characters.
5. Commit the changes locally.
6. Push the branch to remote using `git push ...

### Prompt 9

run golangci-lint
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint config path] in [/home/runner/work/axe/axe] ...
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint config verify] in [/home/runner/work/axe/axe] ...
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint run] in [/home/runner/work/axe/axe] ...
  Error: internal/tool/url_fetch.go:85:23: Error return value of `resp.Body.Close` is not checked (errcheck)
  	defer resp.Body....

### Prompt 10

install it with `go`

### Prompt 11

run golangci-lint
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint config path] in [/home/runner/work/axe/axe] ...
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint config verify] in [/home/runner/work/axe/axe] ...
  Running [/home/runner/golangci-lint-2.10.1-linux-amd64/golangci-lint run] in [/home/runner/work/axe/axe] ...
  Error: internal/tool/url_fetch.go:85:23: Error return value of `resp.Body.Close` is not checked (errcheck)
  	defer resp.Body....

### Prompt 12

commit the changes

### Prompt 13

add `.entire/` to gitignore

### Prompt 14

comit the changes

