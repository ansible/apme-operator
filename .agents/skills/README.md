# APME Operator Agent Skills

Agent skills for pull requests, quality gates, and CI for this Kubebuilder /
Operator SDK project.

Copied and adapted from [ansible/apme](https://github.com/ansible/apme)
`.agents/skills/`. APME-specific SDLC skills (`req-new`, `adr-new`, `tox`, etc.)
were **not** copied — this repo has no `.sdlc/` tree and uses `make` instead of
`tox`.

## Available Skills

### Pull Requests (`pr-*`)

| Skill | Purpose | Arguments |
|-------|---------|-----------|
| `pr-new` | Prepare and submit a pull request | `[branch-name] [--title 'PR title']` |
| `pr-address-feedback` | Handle PR review feedback | `<PR number>` |
| `pr-contributor-review` | Review and prepare a contributor's PR | `<PR number or URL>` |

### Utilities

| Skill | Purpose | Arguments |
|-------|---------|-----------|
| `make` | Makefile target reference (lint, test, deploy) | `[target]` |
| `lean-ci` | GitHub Actions workflow guidance | `[workflow-name]` |
| `branch-align` | Rename branch after artifact renumbering | `[new-branch-name]` |

## Skill Structure

```
.agents/skills/
├── README.md
├── pr-new/
├── pr-address-feedback/
├── pr-contributor-review/
├── branch-align/
├── lean-ci/
└── make/
```

## SKILL.md Format

Each skill has YAML frontmatter (`name`, `description`, `argument-hint`,
`user-invocable`, `metadata`).

## Version

- **Author**: APME Operator contributors (adapted from APME Team skills)
- **License**: Apache 2.0
