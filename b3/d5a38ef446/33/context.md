# Session Context

## User Prompts

### Prompt 1

{miles}=@docs/plans/000_url_fetch_timeout_and_html_stripping_milestones.md

if {miles} is empty or blank, use current context
else create a spec file for the next incomplete milestone in {miles} and it to the top of the new spec document.

## Objective
Create a specification document for a single milestone.
**DO NOT do the work.** ONLY produce the specification document as defined below.

## Requirements
- Use clear, unambiguous language. No assumptions; be explicit.
- All edge cases must be ...

### Prompt 2

We need `029_http_request_timeout_spec.md` to live in @docs/plans/

### Prompt 3

## Objective
Create an implementation guide from a specification document.
**DO NOT do the work.** ONLY produce the implementation guide as defined below.

## Requirements
- If @docs/plans/029_http_request_timeout_spec.md is empty or omitted, pull the latest `*_spec.md` from `docs/plans/` and verify it with the user.
- Read the spec's **Context & Constraints** before writing tasks. Do not re-derive or re-evaluate decisions already made there.
- Each task must reference the exact file path and...

