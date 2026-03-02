# 022 — Docker Containerization Implementation Checklist

Spec: `022_docker_containerization_spec.md`

---

## Phase 1: `.dockerignore`

- [x] **1.1** Create `.dockerignore` at the repository root with the exclusion list from the spec: `.git`, `.github`, `.goreleaser.yml`, `.opencode`, `docs`, `*.md`, with negation patterns `!go.mod`, `!go.sum`, `!skills/sample/SKILL.md`.

## Phase 2: `Dockerfile`

- [x] **2.1** Create `Dockerfile` at the repository root. Add the build stage: base `golang:1.24-alpine`, workdir `/build`, copy `go.mod`/`go.sum` first and run `go mod download`, then copy all source. Set `CGO_ENABLED=0`. Build command: `go build -trimpath -ldflags="-s -w" -o /build/axe .`
- [x] **2.2** Add the runtime stage: base `alpine:3.21`. Install `ca-certificates` only. No git, curl, or other utilities.
- [x] **2.3** Add user setup: create `axe` user/group with UID/GID `10001`. Create directories `/home/axe/.config/axe`, `/home/axe/.local/share/axe`, `/tmp/axe` owned by `axe:axe`. Set `HOME=/home/axe`. Set `USER axe`.
- [x] **2.4** Set environment variables: `HOME=/home/axe`, `XDG_CONFIG_HOME=/home/axe/.config`, `XDG_DATA_HOME=/home/axe/.local/share`.
- [x] **2.5** Copy binary from build stage to `/usr/local/bin/axe`. Set `ENTRYPOINT ["/usr/local/bin/axe"]`. No `CMD`.
- [x] **2.6** Add OCI labels: `org.opencontainers.image.title=axe`, `org.opencontainers.image.description=Lightweight CLI for running single-purpose LLM agents`, `org.opencontainers.image.source=https://github.com/jrswab/axe`, `org.opencontainers.image.licenses=Apache-2.0`.

## Phase 3: `docker-compose.yml`

- [x] **3.1** Create `docker-compose.yml` at the repository root. Define the `axe` service: build context `.`, dockerfile `Dockerfile`, profiles `["cli"]`.
- [x] **3.2** Add `axe` service environment variables: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (from host), `AXE_OLLAMA_BASE_URL=http://ollama:11434`.
- [x] **3.3** Add `axe` service volumes: `${AXE_CONFIG_PATH:-./axe-config}:/home/axe/.config/axe` and named volume `axe-data:/home/axe/.local/share/axe`.
- [x] **3.4** Add `axe` service security hardening: `read_only: true`, `tmpfs: /tmp/axe` (size 10M), `cap_drop: ["ALL"]`, `security_opt: ["no-new-privileges:true"]`.
- [x] **3.5** Add `axe` service networking: `depends_on: ollama` (condition `service_started`), network `axe-net`.
- [x] **3.6** Define the `ollama` service: image `ollama/ollama:latest`, profiles `["ollama"]`, ports `11434:11434`, volume `ollama-data:/root/.ollama`, network `axe-net`.
- [x] **3.7** Define named volumes `axe-data` and `ollama-data`. Define network `axe-net` (bridge).

## Phase 4: Manual Verification

- [x] **4.1** `docker build -t axe .` succeeds on linux/amd64.
- [x] **4.2** `docker run --rm axe version` prints version and exits 0.
- [x] **4.3** `docker run --rm axe run nonexistent` exits with code 1 or 2.
- [ ] **4.4** `docker run --rm -v ./test-config:/home/axe/.config/axe -e ANTHROPIC_API_KEY axe run test-agent` runs an agent and prints output to stdout.
- [ ] **4.5** `echo "hello" | docker run --rm -i -v ./test-config:/home/axe/.config/axe -e ANTHROPIC_API_KEY axe run test-agent` accepts stdin.
- [ ] **4.6** `docker compose --profile ollama up -d ollama` starts the Ollama service.
- [ ] **4.7** `docker compose --profile cli run --rm axe version` prints version via compose.
- [x] **4.8** Container runs as UID 10001: `docker run --rm --entrypoint id axe` confirms.
- [ ] **4.9** Multi-arch build: `docker buildx build --platform linux/amd64,linux/arm64 -t axe:latest .` completes without error.
