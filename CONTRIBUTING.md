# Contributing to APME Operator

Thanks for contributing. Questions not covered here? Open an issue at
[github.com/ansible/apme-operator/issues](https://github.com/ansible/apme-operator/issues).

## Before you submit

- Open pull requests against `main`.
- Prefer a clear, focused change set (conventional commits are welcome, e.g. `fix:`, `feat:`, `docs:`).
- Follow the [Ansible code of conduct](https://docs.ansible.com/ansible/latest/community/code_of_conduct.html).

## Development setup

See [docs/development.md](docs/development.md) for prerequisites, build/deploy, layout, and Makefile targets.

Optional git hooks (same checks as CI lint):

```sh
uv tool install prek && prek install
```

## Submitting changes

1. Fork and create a branch from `main` (or from `upstream/main` if you use a fork + upstream remote).
2. Make your changes.
3. Run quality gates:

   ```sh
   make lint
   make test
   ```

   After `api/` or kubebuilder marker changes, also run `make manifests generate` and commit the generated CRD/RBAC updates with your Go changes.
4. Push and open a PR against [ansible/apme-operator](https://github.com/ansible/apme-operator).

Fork workflow example:

```sh
git fetch upstream
git checkout -b my-change upstream/main
# ... commit ...
git push -u origin HEAD
gh pr create --repo ansible/apme-operator --head <your-user>:<branch> --base main
```

## Testing

| Gate | Command | Required |
|------|---------|----------|
| Lint | `make lint` | Yes |
| Unit / envtest | `make test` | Yes |
| E2E | `make test-e2e` | When changing install/reconcile paths that need a cluster |

## Reporting issues

File bugs and feature requests at
[github.com/ansible/apme-operator/issues](https://github.com/ansible/apme-operator/issues).

Security vulnerabilities: see [.github/SECURITY.md](.github/SECURITY.md) — do **not** report them in public issues.

## Getting help

- [Ansible Forum](https://forum.ansible.com)
- [Development guide](docs/development.md)
- [User guide](docs/user-guide.md)
