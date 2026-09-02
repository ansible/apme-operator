# APME Operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Tests](https://github.com/ansible/apme-operator/actions/workflows/test.yml/badge.svg)](https://github.com/ansible/apme-operator/actions/workflows/test.yml)
[![Code of Conduct](https://img.shields.io/badge/code%20of%20conduct-Ansible-yellow.svg)](https://docs.ansible.com/ansible/latest/community/code_of_conduct.html)

A Kubernetes operator for deploying and managing [APME](https://github.com/ansible/apme) on OpenShift, built with [Operator SDK](https://sdk.operatorframework.io/) / [Kubebuilder](https://book.kubebuilder.io/) (Go).

The operator reconciles a namespaced `Apme` custom resource into the **Simple** all-in-one topology (Gateway and UI share the engine Deployment), with **Postgres-only** persistence — either a managed single-replica StatefulSet or an external connection Secret. Operands default to NetworkPolicies and restricted SCC–friendly pod specs.

## Scope (v1)

| Included | Not in v1 |
|----------|-----------|
| Simple topology (`status.topology: Simple`) | Split / Gateway-outside topology |
| Managed or external Postgres | Backup / Restore CRDs |
| OpenShift Routes (default) and optional Ingress | Multi-replica Simple deployments |

## Quick start

Requires `kubectl` (or `oc`) and a cluster. OpenShift is the primary target (Routes); Ingress works on vanilla Kubernetes.

### Install from a release (preferred)

```sh
kubectl apply -f https://github.com/ansible/apme-operator/releases/latest/download/install.yaml

kubectl create namespace apme --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n apme -f https://raw.githubusercontent.com/ansible/apme-operator/main/config/samples/apme_v1alpha1_apme.yaml
```

Pin a version with `…/releases/download/vX.Y.Z/install.yaml`. See the [user guide](docs/user-guide.md#install-the-operator).

### Build and deploy from source

```sh
export IMG=quay.io/$USER/apme-operator:dev
make docker-build docker-push deploy IMG=$IMG

kubectl create namespace apme --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n apme -f config/samples/apme_v1alpha1_apme.yaml
```

Optionally set `spec.exposure.route.host` in the sample to a custom hostname (requires `routes/custom-host` on the operator SA, which the install manifests grant). Omit `host` to use the OpenShift default.

Check status:

```sh
kubectl get apme -n apme
kubectl get pods -n apme
```

CRDs only: `make install`. Tear down: delete the `Apme` CR (owned objects GC), then remove the install manifest or `make undeploy` / `make uninstall`.

## Documentation

| Doc | Contents |
|-----|----------|
| [User guide](docs/user-guide.md) | CR fields, database modes, exposure, samples |
| [Development](docs/development.md) | Prerequisites, make targets, layout, tests |
| [Contributing](CONTRIBUTING.md) | PR workflow and quality gates |
| [Security](.github/SECURITY.md) | Vulnerability reporting |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [development guide](docs/development.md).

We ask contributors to follow the [Ansible code of conduct](https://docs.ansible.com/ansible/latest/community/code_of_conduct.html).

## Get help

- Issues: [github.com/ansible/apme-operator/issues](https://github.com/ansible/apme-operator/issues)
- Forum: [forum.ansible.com](https://forum.ansible.com)
- APME product: [github.com/ansible/apme](https://github.com/ansible/apme)
