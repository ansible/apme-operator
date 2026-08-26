# APME Operator

Go operator (Operator SDK / Kubebuilder) that installs [APME](https://github.com/ansible/apme) on OpenShift as a **Simple** all-in-one pod (ADR-069): Gateway and UI share the engine Deployment. Persistence is **Postgres only** ([APME #543](https://github.com/ansible/apme/pull/543)): a managed single-replica StatefulSet, or an external Secret.

v1 does **not** wrap Helm, does not use SQLite, and does not implement Split (Gateway-outside) topology. Helm in `ansible/apme` remains the documented K8s path until this operator actually installs. Plan: [issue #2](https://github.com/ansible/apme-operator/issues/2).

## CR

```yaml
apiVersion: apme.ansible.com/v1alpha1
kind: Apme
metadata:
  name: apme
spec:
  version: "2026.8.6"
  replicas: 1                 # CEL max 1 in v1
  exposure:
    route:
      enabled: true
      host: apme.apps.example.com   # required when UI is on
  # database.connectionSecretRef unset => operator creates Postgres
  abbenay:
    enabled: false
```

External database: set `spec.database.connectionSecretRef.name` to a Secret whose key (default `database-url`) is a `postgresql+asyncpg://` URL. Switching from managed to external **does not delete** the StatefulSet/PVC.

## Prerequisites

- Go 1.24+
- `kubectl` and an OpenShift project (Routes) or a cluster with Ingress
- Container runtime for `make docker-build`

Operator SDK v1.42.3 was used to scaffold. Day-to-day: `make`.

## Deploy (inner loop)

```sh
export IMG=quay.io/$USER/apme-operator:dev
make docker-build docker-push deploy IMG=$IMG
kubectl apply -f config/samples/apme_v1alpha1_apme.yaml
```

`scripts/operator.mk` defines `NAMESPACE`, `QUAY_USER` / `IMAGE`, and `DEV_CR`.

Install CRDs only: `make install`. OLM bundle: `make bundle`.

Uninstall: delete the `Apme` CR (owner refs garbage-collect children), then `make undeploy` / `make uninstall`.

## Tests

```sh
make test
make lint
```

envtest covers managed Postgres, external Secret, Abbenay on/off, and mode-switch (no delete of leftover Postgres).

## Layout

```
api/v1alpha1/          Apme CRD
internal/controller/   thin reconciler (SSA)
internal/resolve/      CR → Desired (defaults, db mode)
internal/manifests/    SA, PVC, Services, Routes, Ingress, NP, Deployment
  containers/          one builder per APME container
  postgres/            StatefulSet + generated Secret
```
