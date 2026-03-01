# Implementation Checklist: Integration Test Infrastructure (Phase 1)

**Spec:** `docs/plans/008_i9n_test_infrastructure_spec.md`
**Status:** Complete

---

## 1. Fixture Agent Configurations

- [x] Create directory `cmd/testdata/agents/`
- [x] Create `cmd/testdata/agents/basic.toml` with fields: `name = "basic"`, `model = "openai/gpt-4o"` (Req 1.2)
- [x] Create `cmd/testdata/agents/with_skill.toml` with fields: `name`, `model`, `skill = "skills/stub/SKILL.md"` (Req 1.3)
- [x] Create `cmd/testdata/agents/with_files.toml` with fields: `name`, `model`, `files = ["README.md", "docs/**/*.md"]` (Req 1.4)
- [x] Create `cmd/testdata/agents/with_memory.toml` with fields: `name`, `model`, `[memory]` section (`enabled`, `last_n`, `max_entries`; no `path`) (Req 1.5)
- [x] Create `cmd/testdata/agents/with_subagents.toml` with fields: `name`, `model`, `sub_agents`, `[sub_agents_config]` section (Req 1.6)

## 2. Stub Skill File

- [x] Create directory `cmd/testdata/skills/stub/`
- [x] Create `cmd/testdata/skills/stub/SKILL.md` with sections: title, Purpose, Instructions, Output Format (Req 2.1)
- [x] Verify existing `cmd/testdata/skills/sample/SKILL.md` is untouched (Req 2.2)

## 3. Test Helper Package Skeleton

- [x] Create directory `internal/testutil/`
- [x] Create `internal/testutil/testutil.go` with `package testutil` declaration and necessary imports (Req 3.1)
- [x] Create `internal/testutil/testutil_test.go` with `package testutil` declaration

## 4. SetupXDGDirs — Tests First (Red)

- [x] Write `TestSetupXDGDirs_CreatesDirectoryStructure` — verify `configDir` contains `agents/` and `skills/`, `dataDir` exists, env vars point to parents (Spec 7.2)
- [x] Write `TestSetupXDGDirs_EnvVarsSet` — verify `XDG_CONFIG_HOME` and `XDG_DATA_HOME` are absolute paths (Spec 7.2)
- [x] Write `TestSetupXDGDirs_NoFilesCreated` — walk trees, verify only directories (Spec 7.2)

## 5. SetupXDGDirs — Implementation (Green)

- [x] Implement `SetupXDGDirs(t *testing.T) (configDir, dataDir string)` per Req 3.2 and 3.3
- [x] Run `make test` — all three SetupXDGDirs tests pass, no existing tests broken

## 6. SeedFixtureAgents — Tests First (Red)

- [x] Write `TestSeedFixtureAgents_CopiesAllTomlFiles` — create temp source with `.toml` and `.txt` files, verify only `.toml` copied byte-identical (Spec 7.2)
- [x] Write `TestSeedFixtureAgents_SrcDirNotExist` — verify fatal behavior on missing source dir (Spec 7.2)

## 7. SeedFixtureAgents — Implementation (Green)

- [x] Implement `SeedFixtureAgents(t *testing.T, srcDir, dstAgentsDir string)` per Req 4.1 and 4.2
- [x] Run `make test` — SeedFixtureAgents tests pass

## 8. SeedFixtureSkills — Tests First (Red)

- [x] Write `TestSeedFixtureSkills_CopiesRecursively` — nested `stub/SKILL.md` and `advanced/SKILL.md`, verify both copied with correct content (Spec 7.2)

## 9. SeedFixtureSkills — Implementation (Green)

- [x] Implement `SeedFixtureSkills(t *testing.T, srcDir, dstSkillsDir string)` per Req 4.3 and 4.4
- [x] Run `make test` — SeedFixtureSkills test passes

## 10. SeedGlobalConfig — Tests First (Red)

- [x] Write `TestSeedGlobalConfig_WritesConfigToml` — verify file exists with exact content (Spec 7.2)
- [x] Write `TestSeedGlobalConfig_OverwritesExisting` — call twice, verify last content wins (Spec 7.2)

## 11. SeedGlobalConfig — Implementation (Green)

- [x] Implement `SeedGlobalConfig(t *testing.T, configDir, content string)` per Req 6.1
- [x] Run `make test` — SeedGlobalConfig tests pass

## 12. BuildBinary — Tests First (Red)

- [x] Write `TestBuildBinary_ProducesBinary` — verify file exists, is executable, runs `version` successfully (Spec 7.2)
- [x] Write `TestBuildBinary_ReturnsSamePathOnSecondCall` — call twice, verify same path (Spec 7.2)
- [x] Write `TestCleanupBinary_RemovesCacheDir` — call BuildBinary then CleanupBinary, verify dir gone (Spec 7.2)

## 13. BuildBinary — Implementation (Green)

- [x] Implement module root discovery: walk up from testutil package dir to find `go.mod` (Req 5.4)
- [x] Implement `BuildBinary(t *testing.T) string` with `sync.Once`, `os.MkdirTemp`, platform-aware binary name (Req 5.1, 5.2, 5.5)
- [x] Implement `CleanupBinary()` (Req 5.3)
- [x] Run `make test` — BuildBinary and CleanupBinary tests pass

## 14. Fixture Validation Tests (Red then Green)

- [x] Write `TestFixtureAgents_AllParseAndValidate` — loop over `cmd/testdata/agents/*.toml`, parse and validate each (Spec 7.3)
- [x] Write `TestFixtureAgents_BasicHasOnlyRequiredFields` — verify zero-valued optional fields (Spec 7.3)
- [x] Write `TestFixtureAgents_WithSkillReferencesStubSkill` — verify `Skill` field value (Spec 7.3)
- [x] Write `TestFixtureAgents_WithMemoryConfig` — verify all memory fields (Spec 7.3)
- [x] Write `TestFixtureAgents_WithSubagentsConfig` — verify sub_agents list and config fields (Spec 7.3)
- [x] Write `TestFixtureSkills_StubSkillExists` — read stub SKILL.md, verify sections (Spec 7.3)
- [x] Run `make test` — all fixture validation tests pass

## 15. Final Verification

- [x] Run `make test` — full suite passes with zero failures (Acceptance Criterion 1)
- [x] Verify `go.mod` has no new dependencies (Acceptance Criterion 12)
- [x] Verify `internal/testutil/` is not imported by any non-test Go file (Acceptance Criterion 13)
- [x] Update milestone checklist in `docs/plans/000_i9n_milestones.md`: mark Phase 1 items `[-]` or `[x]` as appropriate
