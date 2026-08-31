---
name: make
description: >
  Reference for running lint, test, generate, and image build via the Makefile.
  Agents MUST use make targets for quality gates — do not invoke golangci-lint,
  go test, or controller-gen directly unless a skill explicitly allows it for
  debugging. This skill is the canonical lookup for which target to use.
argument-hint: "[target]"
user-invocable: true
metadata:
  author: APME Operator contributors
  version: 1.0.0
---

# make — Developer & Agent Orchestration

## Usage

```
/make                  # Show this reference (read SKILL.md)
/make lint             # golangci-lint
/make test             # envtest unit tests
/make generate         # DeepCopy + controller-gen
```

The root `Makefile` is the primary orchestration layer. CI runs the same
commands developers run locally.

## Hard Rules

1. **Quality gates before PR:** `make lint` and `make test` (see `pr-new`).
2. **Do not skip `go mod tidy`** when CI does it (`test.yml` runs tidy before test).
3. **After API or marker changes:** `make manifests generate` then commit CRD/RBAC
   updates together with Go changes.
4. **Image builds:** `make docker-build IMG=...` or OpenShift `BuildConfig` in lab
   gitops; Dockerfile uses UBI go-toolset + ubi-minimal.

## Target Reference

### Quality gates (run before every PR)

| Target | What it runs | When to use |
|--------|--------------|-------------|
| `make lint` | `golangci-lint run` (via `bin/golangci-lint`) | Before every commit/PR |
| `make test` | envtest + `go test` (excludes `./test/e2e`) | After controller/manifest changes |
| `make fmt` | `go fmt ./...` | Optional; lint may expect formatted code |
| `make vet` | `go vet ./...` | Included in `make test` recipe chain |

### Code generation

| Target | What it runs | When to use |
|--------|--------------|-------------|
| `make generate` | controller-gen object DeepCopy | After `api/` type changes |
| `make manifests` | CRD, RBAC, webhook scaffolding | After kubebuilder markers / RBAC comments change |

`make test` and `make build` depend on `manifests generate fmt vet`.

### Build & image

| Target | What it runs | When to use |
|--------|--------------|-------------|
| `make build` | `go build` → `bin/manager` | Quick local compile |
| `make docker-build` | `docker build` / `podman build` | Local image (`IMG=...`) |
| `make docker-push` | Push `IMG` | After docker-build |
| `make docker-buildx` | Multi-arch buildx push | Release images |
| `make build-installer` | kustomize → `dist/install.yaml` | Offline install bundle |

### Cluster deploy (inner loop)

| Target | What it runs | When to use |
|--------|--------------|-------------|
| `make install` | CRDs only | First-time CRD install |
| `make deploy` | CRD + RBAC + manager Deployment | Dev cluster (`IMG=...`) |
| `make undeploy` | Remove manager | Tear down operator |
| `make uninstall` | Remove CRDs | Destructive |

See also `scripts/operator.mk` (`NAMESPACE`, `QUAY_USER`, `DEV_CR`).

### E2E (optional, slower)

| Target | What it runs | When to use |
|--------|--------------|-------------|
| `make test-e2e` | Kind + ginkgo e2e | Full cluster smoke (CI: `test-e2e.yml`) |

## Common Agent Workflows

### "I changed controller or manifests"

```bash
make manifests generate   # if api/ or +kubebuilder markers changed
make lint
make test
```

### "I changed only Dockerfile or manager Deployment resources"

```bash
make lint                 # if Go unchanged, still run if any .go touched
make docker-build IMG=quay.io/$USER/apme-operator:dev
```

### "List all targets"

```bash
make help
```

## CI parity

| Workflow | Command |
|----------|---------|
| `.github/workflows/prek.yml` | `make golangci-lint` + prek |
| `.github/workflows/test.yml` | `go mod tidy && make test` |
| `.github/workflows/test-e2e.yml` | `make test-e2e` |
| `.github/workflows/release.yml` | multi-arch image push + `make build-installer` |
