# User guide

Install and configure APME with the `Apme` custom resource.

## Prerequisites

- A Kubernetes cluster with `kubectl` (or OpenShift with `oc`)
- The APME Operator deployed in the cluster (see [Install the operator](#install-the-operator) or the [development guide](development.md#build-and-deploy))
- For OpenShift Routes: a project/namespace; a custom hostname is optional
- For external Postgres: a Secret with a `postgresql+asyncpg://` URL

## Install the operator

Prefer a published release (no local image build):

```sh
kubectl apply -f https://github.com/ansible/apme-operator/releases/latest/download/install.yaml
```

Pin a version when you need a fixed install:

```sh
kubectl apply -f https://github.com/ansible/apme-operator/releases/download/v0.1.0/install.yaml
```

Each GitHub Release attaches `install.yaml` with the controller image pinned to that tag
(`ghcr.io/ansible/apme-operator:<version>`). For local iteration, use the
[development build-and-deploy](development.md#build-and-deploy) path.

## Deploy an Apme instance

Minimal managed-Postgres example (OpenShift Route):

```yaml
apiVersion: apme.ansible.com/v1alpha1
kind: Apme
metadata:
  name: apme
spec:
  version: "2026.8.10"
  replicas: 1                 # max 1 in v1
  exposure:
    route:
      enabled: true
      # host: apme.apps.example.com   # optional; omit to use OpenShift default host
  # database.connectionSecretRef unset => operator creates Postgres
  abbenay:
    enabled: false
```

Apply (with a local checkout):

```sh
kubectl create namespace apme --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n apme -f config/samples/apme_v1alpha1_apme.yaml
```

Or without a checkout:

```sh
kubectl apply -n apme -f https://raw.githubusercontent.com/ansible/apme-operator/main/config/samples/apme_v1alpha1_apme.yaml
```

Samples in-repo:

| File | Use |
|------|-----|
| [`config/samples/apme_v1alpha1_apme.yaml`](../config/samples/apme_v1alpha1_apme.yaml) | Managed Postgres + Route |
| [`config/samples/apme_v1alpha1_apme_external.yaml`](../config/samples/apme_v1alpha1_apme_external.yaml) | External DB Secret |

## Spec overview

| Field | Default / notes |
|-------|-----------------|
| `version` | APME **calendar** image tag (default `2026.8.10`), not a git SHA and not the operator release tag |
| `image.registry` | `quay.io/ansible`; images are `{registry}/apme-{name}:{version}` |
| `replicas` | `1` (maximum 1 in v1) |
| `components.*` | Optional toggles (`gitleaks`, `collectionHealth`, `depAudit`, `ui`); omitted booleans default **true** |
| `database` | Managed Postgres unless `connectionSecretRef.name` is set |
| `storage` | PVC sizes for sessions and Galaxy proxy cache |
| `exposure.route` | Enabled by default on OpenShift; `host` optional (OpenShift default when empty) |
| `exposure.ingress` | Optional vanilla Kubernetes Ingress |
| `abbenay` | AI sidecar; **off** by default |
| `resources` | Optional floor overrides applied to every APME container |
| `networkPolicy` | Enabled by default |

Full schema: [`api/v1alpha1/apme_types.go`](../api/v1alpha1/apme_types.go) and the generated CRD under `config/crd/`.

### Images and tags

The operator pulls one shared tag for all APME containers:

`{spec.image.registry}/apme-{component}:{spec.version}`

Examples with defaults: `quay.io/ansible/apme-engine:2026.8.10`, `…/apme-ui:2026.8.10`, `…/apme-gateway:2026.8.10`.

- Tags are **APME calendar releases** published on Quay under `quay.io/ansible/apme-*`.
- They are **not** operator git tags (`v0.1.0`) and **not** Helm chart SHA digests unless that digest is also tagged on the Quay `apme-*` repositories.
- Set `spec.version` to a tag that exists on Quay for your registry; use `spec.image.pullSecrets` / a mirror registry when pulling privately.
- If a tag is missing, pods show `ImagePullBackOff` and the `Apme` status surfaces an `ImagePullError` / waiting message with the resolved image name.

## Database

### Managed (default)

Leave `spec.database.connectionSecretRef` unset. The operator creates a single-replica Postgres StatefulSet, Service, Secret, PVC, and NetworkPolicy. Override image, storage, or resources under `spec.database.postgres`.

### External

Point at an existing Secret:

```yaml
spec:
  database:
    connectionSecretRef:
      name: apme-external-db
      key: database-url          # default if omitted
```

The Secret value must be a `postgresql+asyncpg://` URL (same contract as [APME #543](https://github.com/ansible/apme/pull/543)).

Switching from managed to external **does not delete** the leftover StatefulSet/PVC. Delete those objects yourself if you no longer need them.

## Exposure

- **Route (OpenShift):** `spec.exposure.route.enabled` defaults true. Leave `host` empty for the OpenShift default hostname, or set a custom host. Custom hosts require the operator ServiceAccount to have `create` on `route.openshift.io/routes/custom-host` (included in the bundled ClusterRole). If that permission is missing, status sets `Degraded=True` with reason `RoutePermissionDenied`.
- Disabling Routes (`enabled: false`) or turning off `components.ui` deletes only Routes owned by the `Apme` CR. Changing `host` recreates owned Routes so the new host converges without manual deletion.
- **Ingress:** set `spec.exposure.ingress.enabled: true` plus `host` / `className` / annotations / TLS as needed.

Status surfaces a public URL when ready:

```sh
kubectl get apme apme -n apme -o jsonpath='{.status.url}{"\n"}'
```

Conditions include `Ready`, `Progressing`, `Degraded`, and `DatabaseReady`. Columns: `kubectl get apme`.

## Uninstall

1. Delete the `Apme` CR — owner references garbage-collect owned children.
2. Remove the operator: delete the release install (`kubectl delete -f …/install.yaml`) or `make undeploy`
3. Remove CRDs if desired: `make uninstall` (destructive for all `Apme` instances cluster-wide)

Managed Postgres PVCs may retain data depending on storage class reclaim policy; delete PVCs explicitly if you need a clean slate.
