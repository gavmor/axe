# 022 -- Docker Containerization Spec

## Goal

Provide a Dockerfile, .dockerignore, and docker-compose.yml that allow users to build and run axe in a Docker container. The primary use case is ephemeral CLI runs: `docker run --rm axe run my-agent`. A secondary compose configuration supports running axe alongside a local Ollama instance.

## Scope

### In Scope

- Multi-stage Dockerfile producing a minimal, hardened container image.
- Multi-architecture builds (linux/amd64, linux/arm64).
- `.dockerignore` to keep the build context small.
- `docker-compose.yml` for axe + Ollama sidecar.
- Security hardening (non-root user, read-only rootfs, dropped capabilities).
- Documentation of volume mounts, environment variables, and networking.

### Out of Scope

- CI/CD pipeline integration (GitHub Actions, GitLab CI).
- Publishing the image to a container registry.
- Helm charts, Kubernetes manifests, or any orchestration beyond compose.
- Daemon mode or long-running container patterns (axe is an executor, not a scheduler).
- Changes to axe source code. This spec adds files only; no Go code is modified.

## Files to Create

### 1. `Dockerfile`

Located at the repository root.

### 2. `.dockerignore`

Located at the repository root.

### 3. `docker-compose.yml`

Located at the repository root.

---

## Dockerfile Specification

### Build Stage

- **Base image:** `golang:1.24-alpine`
- **Working directory:** `/build`
- **Copy order (for layer caching):**
  1. `go.mod` and `go.sum` -- run `go mod download`
  2. All remaining source files
- **Build command:** `go build -trimpath -ldflags="-s -w" -o /build/axe .`
- **Environment:** `CGO_ENABLED=0`
- **Rationale:** These flags match the existing `.goreleaser.yml` configuration exactly. `CGO_ENABLED=0` produces a fully static binary.

### Runtime Stage

- **Base image:** `alpine:3.21`
- **Rationale for alpine over scratch:** The `run_command` tool executes commands via `sh -c`. This requires `/bin/sh` to exist in the container. A `scratch` image has no shell. Alpine provides a minimal shell via busybox.
- **Install:** `ca-certificates` only. Required for HTTPS connections to LLM provider APIs (Anthropic, OpenAI). No other packages.
- **Do not install:** git, curl, or other utilities. If a user's agents need additional tools for `run_command`, they must extend the image themselves.

### User Setup

- Create a non-root user and group named `axe` with UID/GID `10001`.
- Create the following directories owned by `axe:axe`:
  - `/home/axe/.config/axe` -- config directory
  - `/home/axe/.local/share/axe` -- data directory
  - `/tmp/axe` -- scratch space for read-only rootfs mode
- Set `HOME=/home/axe` in the image.
- Set `USER axe` as the runtime user.

### Environment Variables (baked into image)

| Variable | Value | Purpose |
|---|---|---|
| `HOME` | `/home/axe` | Required for XDG fallback path resolution |
| `XDG_CONFIG_HOME` | `/home/axe/.config` | Explicit; avoids reliance on HOME fallback |
| `XDG_DATA_HOME` | `/home/axe/.local/share` | Explicit; avoids reliance on HOME fallback |

These are defaults. Users override them at `docker run` time if needed.

### Binary and Entrypoint

- Copy the built binary from the build stage to `/usr/local/bin/axe`.
- Set `ENTRYPOINT ["/usr/local/bin/axe"]`.
- Do not set a `CMD`. The user provides the axe subcommand and arguments directly:
  ```
  docker run --rm axe run my-agent
  docker run --rm axe version
  docker run --rm axe agents list
  ```

### Health Check

- `HEALTHCHECK` is not set. Axe is a CLI tool that runs to completion and exits. A healthcheck on an ephemeral container is meaningless and would prevent `--rm` from cleaning up promptly.

### Labels

Apply the following OCI image labels:

| Label | Value |
|---|---|
| `org.opencontainers.image.title` | `axe` |
| `org.opencontainers.image.description` | `Lightweight CLI for running single-purpose LLM agents` |
| `org.opencontainers.image.source` | `https://github.com/jrswab/axe` |
| `org.opencontainers.image.licenses` | Value from the repository's LICENSE file (read at spec time, do not hardcode) |

---

## .dockerignore Specification

Exclude the following from the Docker build context:

```
.git
.github
.goreleaser.yml
.opencode
docs
*.md
!go.mod
!go.sum
!skills/sample/SKILL.md
```

### Rationale

- `.git` -- large, not needed for build.
- `.github` -- CI workflows, not needed.
- `.goreleaser.yml` -- release config, not needed.
- `.opencode` -- tooling config, not needed.
- `docs` -- documentation, not needed.
- `*.md` with exceptions -- markdown files are not needed except `skills/sample/SKILL.md` which is embedded via `go:embed` in `main.go`. The `!` negation patterns ensure the embedded file and Go module files are included despite earlier exclusions.

---

## docker-compose.yml Specification

### Services

#### `axe`

- **build context:** `.` (repository root)
- **dockerfile:** `Dockerfile`
- **profiles:** `["cli"]`
- **environment variables (example, user overrides):**
  - `ANTHROPIC_API_KEY` -- from host environment or `.env` file
  - `OPENAI_API_KEY` -- from host environment or `.env` file
  - `AXE_OLLAMA_BASE_URL=http://ollama:11434` -- points to the Ollama sidecar service by Docker DNS name
- **volumes:**
  - `${AXE_CONFIG_PATH:-./axe-config}:/home/axe/.config/axe` -- agent TOML files, skills, config.toml
  - `axe-data:/home/axe/.local/share/axe` -- persistent memory data
- **read_only:** `true`
- **tmpfs:** `/tmp/axe` (size 10M) -- scratch space for memory GC temp files
- **cap_drop:** `["ALL"]`
- **security_opt:** `["no-new-privileges:true"]`
- **depends_on:** `ollama` (with condition `service_started`, only when using Ollama profile)
- **networks:** `axe-net`

#### `ollama`

- **image:** `ollama/ollama:latest`
- **profiles:** `["ollama"]`
- **ports:** `11434:11434` (exposed to host for model management via `ollama pull`, etc.)
- **volumes:**
  - `ollama-data:/root/.ollama` -- persistent model storage
- **networks:** `axe-net`

### Named Volumes

- `axe-data` -- persists agent memory across runs
- `ollama-data` -- persists downloaded Ollama models

### Networks

- `axe-net` -- bridge network connecting axe and ollama services

### Usage Patterns

**Run axe with a cloud provider (no Ollama):**
```
docker compose run --rm axe run my-agent
```

**Run axe with Ollama sidecar:**
```
docker compose --profile ollama up -d ollama
docker compose --profile cli run --rm axe run my-agent
```

**Pull an Ollama model:**
```
docker compose --profile ollama exec ollama ollama pull llama3
```

---

## Volume Mount Specification

| Container Path | Purpose | Access Mode | Contents |
|---|---|---|---|
| `/home/axe/.config/axe/` | Config | Read-write | `config.toml`, `agents/*.toml`, `skills/*/SKILL.md` |
| `/home/axe/.local/share/axe/` | Data | Read-write | `memory/<agent>.md` files |

### Why config is read-write, not read-only

The `axe config init` and `axe agents init` commands write files into the config directory. Mounting it read-only would cause these commands to fail. Users who do not need these commands may mount it `:ro` at their discretion.

### Agent workdir

The container is self-contained. The agent workdir defaults to the current working directory inside the container. No host filesystem mounts for workdir are specified in the default configuration. If an agent's TOML sets a `workdir`, that path must exist inside the container.

---

## Security Hardening Specification

### Non-root user

The axe process runs as UID 10001 (user `axe`). This limits blast radius if the LLM uses `run_command` to execute malicious commands.

### Read-only root filesystem

The compose file sets `read_only: true`. The only writable locations are:
- The data volume mount (`/home/axe/.local/share/axe/`)
- The config volume mount (`/home/axe/.config/axe/`)
- The tmpfs at `/tmp/axe` (for memory GC temp files)

### Dropped capabilities

`cap_drop: ["ALL"]` removes all Linux capabilities. Axe needs none of them. It makes outbound HTTPS requests and reads/writes files -- neither requires elevated privileges.

### No new privileges

`security_opt: ["no-new-privileges:true"]` prevents privilege escalation via setuid/setgid binaries.

### What this does NOT protect against

- Network exfiltration. The LLM can use `run_command` to make arbitrary network requests (e.g., `curl`, `wget`). To mitigate, use `--network=none` when the agent only talks to a local Ollama instance accessible via a shared Docker network.
- Resource exhaustion. No CPU or memory limits are specified by default. Users should add `deploy.resources.limits` in compose or `--memory`/`--cpus` flags at `docker run` time.
- Secrets in environment variables. API keys passed via `-e` are visible in `docker inspect`. For production use, Docker secrets or a secrets manager is preferred.

---

## Multi-Architecture Build Specification

The Dockerfile itself is architecture-agnostic (Go cross-compilation is handled by the Go toolchain). Multi-arch images are built using `docker buildx`:

```
docker buildx build --platform linux/amd64,linux/arm64 -t axe:latest .
```

The spec does not prescribe a buildx bake file or CI pipeline for this. The Dockerfile must work correctly when built for either `linux/amd64` or `linux/arm64`.

---

## Environment Variable Reference

All environment variables that affect axe at runtime. Users pass these at `docker run` or in the compose `environment` block.

| Variable | Required | Purpose | Example |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | If using Anthropic | API authentication | `sk-ant-...` |
| `OPENAI_API_KEY` | If using OpenAI | API authentication | `sk-...` |
| `<PROVIDER>_API_KEY` | If using other providers | Generic pattern | `GROQ_API_KEY=gsk-...` |
| `AXE_OLLAMA_BASE_URL` | If using Ollama | Ollama endpoint | `http://ollama:11434` |
| `AXE_ANTHROPIC_BASE_URL` | No | Override Anthropic endpoint | `https://proxy.example.com` |
| `AXE_OPENAI_BASE_URL` | No | Override OpenAI endpoint | `https://proxy.example.com` |
| `XDG_CONFIG_HOME` | No | Override config base path | `/custom/config` |
| `XDG_DATA_HOME` | No | Override data base path | `/custom/data` |

---

## Exit Code Passthrough

Docker must propagate axe's exit codes to the calling process:

| Axe Exit Code | Meaning | Docker Behavior |
|---|---|---|
| 0 | Success | `docker run` exits 0 |
| 1 | Agent error | `docker run` exits 1 |
| 2 | Config error | `docker run` exits 2 |
| 3 | API error | `docker run` exits 3 |

This works automatically with `ENTRYPOINT` (Docker forwards the container process's exit code). No special handling is needed, but this behavior must be verified in testing.

---

## Stdin Piping

Axe accepts stdin when piped. Docker must support this:

```
echo "Summarize this text" | docker run --rm -i axe run my-agent
```

The `-i` flag is required (`--interactive`). Without it, Docker does not connect stdin to the container. This is standard Docker behavior and requires no special Dockerfile configuration.

---

## Edge Cases

### 1. No config volume mounted

If the user runs `docker run --rm axe run my-agent` without mounting a config volume, axe will fail with exit code 2 (config error) because no agent TOML files exist. This is expected behavior -- the error message from axe will tell the user what's wrong.

### 2. Ollama on Docker host (not compose)

If the user runs Ollama directly on the host (not via compose), the base URL must point to the host:
- Linux: `AXE_OLLAMA_BASE_URL=http://host.docker.internal:11434` with `--add-host=host.docker.internal:host-gateway`
- macOS/Windows Docker Desktop: `AXE_OLLAMA_BASE_URL=http://host.docker.internal:11434` (works automatically)

### 3. Memory GC temp files with read-only rootfs

The memory system's `TrimEntries` function creates temp files (`.axe-trim-*.tmp`) in the same directory as the memory file (`$XDG_DATA_HOME/axe/memory/`). Since the data volume is writable, this works correctly even with a read-only rootfs. The `/tmp/axe` tmpfs is provided as additional scratch space.

### 4. `axe agents edit` in container

This command invokes `$EDITOR`. In the container, no editor is installed. The command will fail with an error. This is acceptable -- editing agent configs should be done on the host, not inside the container.

### 5. `axe config init` / `axe agents init` in container

These commands write to the config directory. They work only if the config volume is mounted read-write. Usage:
```
docker run --rm -v ./my-config:/home/axe/.config/axe axe config init
```

### 6. Signal handling

`docker stop` sends SIGTERM, then SIGKILL after the grace period. Axe uses `context.Context` for cancellation in LLM API calls. SIGTERM will interrupt in-flight API requests, which is correct behavior for ephemeral containers.

### 7. DNS resolution for provider APIs

Alpine includes `musl` libc with DNS resolution. Go's `CGO_ENABLED=0` build uses Go's pure-Go DNS resolver, which reads `/etc/resolv.conf` directly. Docker populates `/etc/resolv.conf` in the container. No issues expected, but this is the mechanism.

### 8. Large model responses

The `run_command` tool caps output at 100KB. LLM response output itself has no axe-side cap (it's whatever the provider returns, bounded by `max_tokens`). Stdout in Docker is line-buffered by default, which is correct for axe's output patterns.

---

## Testing Requirements

### Unit tests (none for this spec)

No Go source code is changed. There are no unit tests to write.

### Manual verification checklist

The following must be verified manually after implementation:

1. `docker build -t axe .` succeeds on linux/amd64.
2. `docker run --rm axe version` prints `axe version 1.0.0` and exits 0.
3. `docker run --rm axe run nonexistent` exits with code 1 or 2 (agent not found).
4. `docker run --rm -v ./test-config:/home/axe/.config/axe -e ANTHROPIC_API_KEY axe run test-agent` runs an agent successfully and prints output to stdout.
5. `echo "hello" | docker run --rm -i -v ./test-config:/home/axe/.config/axe -e ANTHROPIC_API_KEY axe run test-agent` accepts stdin.
6. `docker compose --profile ollama up -d ollama` starts the Ollama service.
7. `docker compose --profile cli run --rm axe version` prints version via compose.
8. The container runs as UID 10001 (verify: `docker run --rm axe id`).
9. Multi-arch build: `docker buildx build --platform linux/amd64,linux/arm64 -t axe:latest .` completes without error.
