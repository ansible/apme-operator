# User guide

Install and configure APME with the `Apme` custom resource.

## Prerequisites

- A Kubernetes cluster with `kubectl` (or OpenShift with `oc`)
- The APME Operator deployed in the cluster ([development guide](development.md#build-and-deploy) or your install method)
- For OpenShift Routes: a project/namespace and a hostname for the UI
- For external Postgres: a Secret with a `postgresql+asyncpg://` URL

## Deploy an Apme instance

Minimal managed-Postgres example (OpenShift Route):

```yaml
apiVersion: apme.ansible.com/v1alpha1
kind: Apme
metadata:
  name: apme
spec:
  version: "2026.8.10"
  replicas: 1                 # CEL max 1 in v1
  exposure:
    route:
      enabled: true
      host: apme.apps.example.com   # required when UI and Route are enabled
  # database.connectionSecretRef unset => operator creates Postgres
  abbenay:
    enabled: false
```

Apply:

```sh
kubectl apply -n apme -f config/samples/apme_v1alpha1_apme.yaml
```

Samples in-repo:

| File | Use |
|------|-----|
| [`config/samples/apme_v1alpha1_apme.yaml`](../config/samples/apme_v1alpha1_apme.yaml) | Managed Postgres + Route |
| [`config/samples/apme_v1alpha1_apme_external.yaml`](../config/samples/apme_v1alpha1_apme_external.yaml) | External DB Secret |

## Spec overview

| Field | Default / notes |
|-------|-----------------|
| `version` | APME image tag (default `2026.8.10`) |
| `image.registry` | `quay.io/ansible`; images are `{registry}/apme-{name}:{version}` |
| `replicas` | `1` (maximum 1 in v1) |
| `components.*` | Optional toggles (`gitleaks`, `collectionHealth`, `depAudit`, `ui`); omitted booleans default **true** |
| `database` | Managed Postgres unless `connectionSecretRef.name` is set |
| `storage` | PVC sizes for sessions and Galaxy proxy cache |
| `exposure.route` | Enabled by default on OpenShift; `host` required when UI + Route are on |
| `exposure.ingress` | Optional vanilla Kubernetes Ingress |
| `abbenay` | AI sidecar; **off** by default |
| `resources` | Optional floor overrides applied to every APME container |
| `networkPolicy` | Enabled by default |

Full schema: [`api/v1alpha1/apme_types.go`](../api/v1alpha1/apme_types.go) and the generated CRD under `config/crd/`.

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

- **Route (OpenShift):** `spec.exposure.route.enabled` defaults true. Set `host` when the UI is enabled. TLS termination defaults to `edge` with redirect for insecure traffic.
- **Ingress:** set `spec.exposure.ingress.enabled: true` plus `host` / `className` / annotations / TLS as needed.

Status surfaces a public URL when ready:

```sh
kubectl get apme apme -n apme -o jsonpath='{.status.url}{"\n"}'
```

Conditions include `Ready`, `Progressing`, `Degraded`, and `DatabaseReady`. Columns: `kubectl get apme`.

## Uninstall

1. Delete the `Apme` CR — owner references garbage-collect owned children.
2. Remove the operator: `make undeploy`
3. Remove CRDs if desired: `make uninstall` (destructive for all `Apme` instances cluster-wide)

Managed Postgres PVCs may retain data depending on storage class reclaim policy; delete PVCs explicitly if you need a clean slate.
