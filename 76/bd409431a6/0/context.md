# Session Context

## User Prompts

### Prompt 1

# Implementation Instructions

1. Study the spec markdown file thoroughly
    - located in `./docs/plans/`
    - Use the spec file that starts with 027
    - If 027 is empty, blank, or does not exist; the newest file formatted as `xxx_topic_spec.md`
2. Study implementation plan markdown file thoroughly
    - located in `./docs/plans/`
    - Use the spec file that starts with 027
    - If 027 is empty, blank, or does not exist; the newest file formatted as `xxx_topic_implement.md`
3. Pick the ...

### Prompt 2

are there any unchecked boxes?

### Prompt 3

commit the changes and any untracked files we need

### Prompt 4

1. Verify that we are not on the develop, main, or master branch.
    - If we are on the one of these branch ask what branch name to switch to.
    - Once given the name create a new git branch off of develop using `git checkout -b`
2. Review all changes made against develop.
3. Create a Git message which lists out the changes made in a short paragraph
4. The title should summarize the changes at a high level. Keep it UNDER 50 characters.
5. Commit the changes locally.
6. Push the branch to r...

