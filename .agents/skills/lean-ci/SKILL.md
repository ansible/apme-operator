---
name: lean-ci
description: >
  Guide for writing and modifying GitHub Actions workflows in apme-operator.
  Use when creating CI jobs, changing build steps, or debugging CI failures.
  Enforces thin workflows that mirror local make targets.
argument-hint: "[workflow-name]"
user-invocable: true
metadata:
  author: APME Operator contributors
  version: 1.0.0
---

# Lean CI (apme-operator)

GitHub Actions workflows should mirror what developers run locally. Substantive
logic lives in the `Makefile`; CI invokes `make` or the same underlying tools.

## Principles

1. **Reproducible locally.** If CI runs `make test`, developers run `make test`.
2. **Thin workflows.** Avoid multi-line shell in YAML unless unavoidable.
3. **Version from repo files.** Go version from `go.mod` (`go-version-file`).
   golangci-lint version matches `Makefile` / `GOLANGCI_LINT_VERSION`.
4. **Pin actions to commit SHAs** when touching workflows (tag comment optional
   but recommended for supply-chain hygiene).

## Workflows (`.github/workflows/`)

| Workflow | Job | Local equivalent |
|----------|-----|------------------|
| `prek.yml` | prek + golangci-lint | `prek run` / `make lint` |
| `test.yml` | `go mod tidy` + `make test` | same |
| `test-e2e.yml` | Kind + e2e | `make test-e2e` |
| `release.yml` | multi-arch image + `install.yaml` | `make docker-buildx` + `make build-installer` |

`prek` / `test` / `test-e2e` trigger on `push` and `pull_request`.
`release.yml` triggers on `v*.*.*` tags (and `workflow_dispatch` for an existing tag).

## Release workflow

Publishes `ghcr.io/<owner>/apme-operator` (linux/amd64 + linux/arm64) and a GitHub
Release with `dist/install.yaml`. Also pushes to
`quay.io/<QUAY_NAMESPACE>/apme-operator` when `QUAY_USERNAME` /
`QUAY_PASSWORD` secrets are set (`QUAY_NAMESPACE` defaults to the repo owner;
override with the `QUAY_NAMESPACE` Actions variable).

Operator tags are independent of APME image tags. Bump
`api/v1alpha1.DefaultVersion` when shipping a new default APME pin.

## Rules for modifications

- **DO** add new checks as Makefile targets first, then call them from CI.
- **DO** use `actions/setup-go` with `go-version-file: go.mod`.
- **DO** keep golangci-lint version aligned with `Makefile` (`v2.1.0` today).
- **DO NOT** hardcode Go versions in workflow YAML.
- **DO NOT** duplicate `controller-gen` or `kustomize` install logic in YAML —
  the Makefile already downloads tools to `bin/`.

## Debugging CI

```bash
gh pr checks <N> --json name,state --jq '.[] | select(.state != "SUCCESS")'
gh run view <RUN_ID> --log-failed 2>&1 | tail -80
```

Fix locally with `make lint` / `make test`, push, re-check.
