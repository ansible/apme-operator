---
name: pr-new
description: >
  Prepare and submit a pull request for apme-operator. Syncs with upstream,
  runs make lint and make test, self-reviews the diff, commits with conventional
  commits, and opens a PR to ansible/apme-operator via gh. Use when the user
  asks to submit, create, or open a pull request.
argument-hint: "[branch-name] [--title 'PR title']"
user-invocable: true
metadata:
  author: APME Operator contributors
  version: 1.0.0
---

# PR New (apme-operator)

## Workflow

### Step 1: Sync with upstream and create a feature branch

Configure `upstream` to `https://github.com/ansible/apme-operator.git` if missing.

```bash
git fetch upstream
git checkout -b <branch-name> upstream/main
```

Use descriptive names: `feat/ubi-manager-image`, `fix/route-custom-host-rbac`.

If work already exists on another branch, rebase or cherry-pick onto the new branch.

### Step 2: Run quality gates

```bash
make lint
make test
```

Both must pass on the **full tree**. If the branch inherited failures, rebase onto
`upstream/main` first.

After API or kubebuilder marker changes:

```bash
make manifests generate
make lint
make test
```

Do **not** use raw `golangci-lint` or `go test` for PR gates — use `make` targets.
See the `make` skill.

### Step 3: Self-review the diff

**Mandatory.** Review the full branch diff:

```bash
git diff upstream/main...HEAD
```

Read surrounding code for each hunk, not just the diff lines.

**Artifact sweep.** List types in the diff (Go, YAML manifests, CRD, Dockerfile,
Makefile, workflow YAML, Markdown). For each, verify:

1. **Types and contracts** — Go signatures, error handling, nil checks; kubebuilder
   validation markers match runtime behavior; CRD OpenAPI matches `api/v1alpha1` types.
2. **Exposure** — logs, error messages, and RBAC grant minimum needed permissions
   (`routes/custom-host` only when setting Route `spec.host`).
3. **Caller surprise** — reconciler requeue behavior, status patch failures, SSA
   conflicts; public `resolve`/`manifests` helpers behave consistently.
4. **Drift** — comments, README, samples, and generated `config/crd/bases/*` stay
   aligned after renames or behavior changes.
5. **Versions pinned to intent** — `go.mod`, golangci version in Makefile/workflow,
   UBI image tags in Dockerfile, `IMG` defaults in Makefile/samples.
6. **Dead weight** — unused imports, unreachable branches, duplicate manifest logic.
7. **Cross-artifact parity** — `+kubebuilder:rbac` ↔ `config/rbac/role.yaml`;
   probe ports ↔ `cmd/main.go` flags; operand image names ↔ `resolve.Desired.Image()`.
8. **Edge cases** — external vs managed Postgres, Route host permission errors,
   ImagePullBackOff image names, controller OOM at low memory limits.
9. **Operator patterns** — SSA field manager `apme-operator`; owner refs for GC;
   OpenShift Route API optional (`HasRouteAPI`); no client-side apply mixing.

Only proceed after this review.

### Step 3b: Rule of Five (cold multi-agent review)

**Mandatory** for non-trivial PRs. Launch **read-only** subagents with no prior
context. Findings in **≥2 passes** are ship-blockers unless explicitly demoted.

| PR size | Passes |
|---------|--------|
| Small fix, docs, single-file chore | 3 (passes 1, 2, 5) |
| Controller, CRD, manifests, Dockerfile, RBAC | 5 (all) |

**Shared preamble** (every pass):

```text
Review Pass <N> of <TOTAL> for apme-operator PR.
Repository: <absolute path>
Branch: <branch>
Base: upstream/main

Run `git diff upstream/main...HEAD`. Read full changed files. Return ONLY findings:
**[Pass N / severity] path:line — description**
```

**Pass 1 — bugs (fast tier):** logic errors, nil deref, wrong RBAC verb, SSA typos,
probe misconfiguration, leaked goroutines, status update loops.

**Pass 2 — consistency (fast tier):** CRD vs Go types, RBAC drift, README/samples
stale, Makefile vs CI workflow mismatch, duplicate labels/selectors.

**Pass 3 — right thing (strong tier):** does the change match the stated goal?
Kubebuilder vs hand-written YAML tradeoffs honest? v1 scope (Simple topology only)?

**Pass 4 — architecture (strong tier):** reconciler thin, manifests in
`internal/manifests`, resolve defaults in `internal/resolve`; operand lifecycle
and Postgres mode-switch behavior.

**Pass 5 — adversarial (strong tier):** permission denied on Routes, missing
`routes/custom-host`, wrong registry/tag, restricted SCC violations, informer OOM.

Fix ship-blockers, re-run `make lint` and `make test`, re-run Rule of Five until
converged or user accepts follow-ups.

### Step 4: Update documentation

| Doc | When |
|-----|------|
| `README.md` | Deploy paths, CR fields, inner-loop commands |
| `config/samples/*.yaml` | New CR fields or defaults |
| `scripts/operator.mk` | Dev workflow variables |

Skip APME-style SDLC docs — this repo has no `.sdlc/`.

### Step 5: Commit with conventional commits

```
<type>[optional scope]: <description>
```

| Type | When |
|------|------|
| `feat` | New reconciler behavior, CRD field, manifest type |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `test` | Tests only |
| `build` | Dockerfile, image, Makefile build targets |
| `ci` | GitHub Actions |
| `chore` | Skills, housekeeping |

Scopes: `controller`, `api`, `manifests`, `resolve`, `crd`, `rbac`, `docker`,
`config`, `ci`, `skills`.

Examples:

- `feat(docker): run manager on UBI 9 minimal`
- `fix(rbac): grant routes/custom-host for Route hosts`
- `chore(skills): add pr-new and make agent skills`

### Step 6: Push and create the pull request

Target **upstream** `ansible/apme-operator` from your fork:

```bash
git push -u origin HEAD

gh pr create --repo ansible/apme-operator \
  --head <your-user>:<branch-name> \
  --base main \
  --title "feat(scope): short title" \
  --body "$(cat <<'EOF'
## Summary
- What changed and why

## Changes
- Notable files or behaviors

## Quality of life
- Skills, CI, docs-only workflow improvements (omit if none)

## Test plan
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make manifests generate` run if API changed
- [ ] Manual: deploy on OCP / Kind if applicable
EOF
)"
```

Return the PR URL.

### Maintaining the PR

When adding commits, update the PR body (`gh pr edit`) so Summary and Test plan
stay current.

For review feedback, use the **`pr-address-feedback`** skill.

### Quality of life section

Use when the PR touches `.agents/skills/`, workflow YAML, or contributor docs —
not production operator code.
