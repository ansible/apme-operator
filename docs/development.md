# Development guide

Build, deploy, and test the APME Operator locally.

## Prerequisites

- Go **1.26+** (see `go.mod`)
- `kubectl` / `oc` and access to a cluster (OpenShift preferred for Routes)
- A container runtime for images (`docker` or `podman`; Makefile uses `CONTAINER_TOOL`)
- Optional: [prek](https://github.com/j178/prek) for git hooks — `uv tool install prek && prek install` (runs the same lint path as CI)

Operator SDK **v1.42.3** was used to scaffold. Day-to-day work is via `make` (see `make help`).

## Build and deploy

```sh
export IMG=quay.io/$USER/apme-operator:dev
make docker-build docker-push deploy IMG=$IMG
```

Install CRDs only: `make install`.

`scripts/operator.mk` wraps a common inner loop (`NAMESPACE`, `QUAY_USER` / `IMAGE`, `DEV_CR`):

```sh
make -f scripts/operator.mk deploy   # build, push, deploy operator, apply DEV_CR
make -f scripts/operator.mk down     # delete CR, undeploy operator
```

Run the manager locally against a configured kubeconfig (no image push):

```sh
make install
make run
```

### Useful variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `IMG` | `controller:latest` | Operator image for build/deploy |
| `CONTAINER_TOOL` | `docker` | Image build/push tool |
| `NAMESPACE` (`operator.mk`) | `apme` | Namespace for the sample CR |
| `DEV_CR` (`operator.mk`) | `config/samples/apme_v1alpha1_apme.yaml` | CR applied by `operator.mk deploy` |

## Quality gates

```sh
make lint
make test
```

After changes under `api/` or kubebuilder markers:

```sh
make manifests generate
make lint
make test
```

| Target | Purpose |
|--------|---------|
| `make lint` | golangci-lint |
| `make test` | envtest unit tests (excludes `./test/e2e`) |
| `make manifests` | CRD / RBAC generation |
| `make generate` | DeepCopy and related codegen |
| `make test-e2e` | Kind + ginkgo e2e (optional / slower) |
| `make bundle` | OLM bundle generation |

envtest coverage includes managed Postgres, external Secret, Abbenay on/off, and managed→external mode switch (leftover Postgres is not deleted).

CI mirrors these targets: `.github/workflows/test.yml`, `prek.yml`,
`test-e2e.yml`, and `release.yml` (on version tags).

## Release

Operator releases are independent of APME image tags. Each operator release pins
a default APME tag via `DefaultVersion` in `api/v1alpha1/apme_types.go`
(override per CR with `spec.version`).

1. Bump `DefaultVersion` (and samples/docs) when adopting a new APME tag.
2. Tag the repo: `git tag v0.1.0 && git push upstream v0.1.0`
3. `.github/workflows/release.yml` builds `linux/amd64` + `linux/arm64`, pushes
   to `ghcr.io/<owner>/apme-operator`, attaches `dist/install.yaml` to a GitHub
   Release, and mirrors to Quay when `QUAY_USERNAME` / `QUAY_PASSWORD` are
   configured.

Local equivalent:

```sh
make docker-buildx IMG=ghcr.io/$USER/apme-operator:0.1.0
make build-installer IMG=ghcr.io/$USER/apme-operator:0.1.0
```

## Project layout

```
api/v1alpha1/          Apme CRD types
cmd/                   manager main
config/                CRD, RBAC, manager, samples, kustomize
internal/controller/   thin reconciler (server-side apply)
internal/resolve/      CR → Desired (defaults, database mode)
internal/manifests/    SA, PVC, Services, Routes, Ingress, NP, Deployment
  containers/          one builder per APME container
  postgres/            StatefulSet + generated Secret
scripts/operator.mk    inner-loop helpers
test/                  e2e and shared test helpers
```

## Agent / automation notes

Repo-specific agent skills live under [`.agents/skills/`](../.agents/skills/). See [AGENTS.md](../AGENTS.md) for quality-gate expectations when opening PRs.
