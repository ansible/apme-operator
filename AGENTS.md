# Agent instructions (apme-operator)

Go Kubebuilder operator for APME Simple topology on OpenShift. See
[README.md](README.md) for CRD, deploy, and test basics.

## Quality gates

Before opening or updating a PR:

```bash
make lint
make test
```

After `api/` or kubebuilder marker changes: `make manifests generate` then re-run
gates. See `.agents/skills/make/SKILL.md`.

## Agent skills

Slash commands and proactive workflows live in `.agents/skills/`. When the user
invokes `/pr-new`, `/pr-address-feedback`, or similar, read the matching
`SKILL.md` **before** acting.

| Skill | Use when |
|-------|----------|
| `pr-new` | Create or submit a PR to `ansible/apme-operator` |
| `pr-address-feedback` | Respond to review comments, resolve threads |
| `pr-contributor-review` | Help merge-ready a contributor's PR |
| `make` | Which Makefile target to run |
| `lean-ci` | Change or debug GitHub Actions |
| `branch-align` | Rename branch to match work after re-scope |

Skills adapted from [ansible/apme](https://github.com/ansible/apme)
`.agents/skills/` (PR workflow and CI). APME SDLC skills (`req-new`, `adr-new`,
`tox`, etc.) were not copied — this repo has no `.sdlc/` tree.

## Upstream PRs

Fork workflow: `git fetch upstream`, branch from `upstream/main`, push to your
fork, then:

```bash
gh pr create --repo ansible/apme-operator --head <user>:<branch> --base main
```
