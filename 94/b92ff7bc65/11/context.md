# Session Context

## User Prompts

### Prompt 1

Go read https://github.com/jrswab/axe/issues/24 and let's plan out how we can add this feature.

### Prompt 2

per "call_agent — unchanged. Sub-agents get their own allowed_hosts from their own TOML." does this mean if the axe running the request spins up sub-agents those agents call whatever? If so we need to make sure any sub-agents that are called by the axe instance follows the allow-list from the toml file as they don't get their own toml files.

### Prompt 3

When sub-agent field is defined we should defer to that sub-agent allow list. This way the user can define exactly what each agent (sub or not) is allowed to fetch.

### Prompt 4

{miles}=

if {miles} is empty or blank
    use current context
else
    create a spec file for the next incomplete milestone in {miles}
    and add the milestone document path to the top of the new spec document.

## Objective
Create a specification document for a single milestone.
**DO NOT do the work.** ONLY produce the specification document as defined below.

## Requirements
- Use clear, unambiguous language. No assumptions; be explicit.
- All edge cases must be documented.
- Ask question...

### Prompt 5

## Objective
Create an implementation guide from a specification document.
**DO NOT do the work.** ONLY produce the implementation guide as defined below.

## Requirements
- If docs/plans/040_output_allowlist_spec.md is empty or omitted, pull the latest `*_spec.md` from `docs/plans/` and verify it with the user.
- Read the spec's **Context & Constraints** before writing tasks. Do not re-derive or re-evaluate decisions already made there.
- Each task must reference the exact file path and func...

### Prompt 6

# Code Generation Instructions

Before starting, ask what branch to use.

## Setup
Locate the plan files in `docs/plans/`:
- Spec: file starting with 040, or the newest `xxx_topic_spec.md` if 040 is empty.
- Implement: file starting with 040, or the newest `xxx_topic_implement.md` if 040 is empty.

## Steps

1. Read the **Context Summary** in the implement guide to understand the *why* before selecting any tasks.
2. Study the spec file thoroughly.
3. Study the implement file thoroughly.
4. Pi...

### Prompt 7

# Code Generation Instructions

Before starting, ask what branch to use.

## Setup
Locate the plan files in `docs/plans/`:
- Spec: file starting with 040, or the newest `xxx_topic_spec.md` if 040 is empty.
- Implement: file starting with 040, or the newest `xxx_topic_implement.md` if 040 is empty.

## Steps

1. Read the **Context Summary** in the implement guide to understand the *why* before selecting any tasks.
2. Study the spec file thoroughly.
3. Study the implement file thoroughly.
4. Pi...

