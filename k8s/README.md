# Kubernetes manifests

**Do not hand-edit YAML for the application stack here.** The canonical definition is the Helm chart under [`charts/e-team/`](../charts/e-team/).

## Static manifests (kubectl apply)

Rendered manifests are checked in under [`helm-rendered/`](./helm-rendered/e-team.yaml). Regenerate after changing the chart or `values.yaml`:

```bash
./scripts/render-k8s-manifests.sh
```

Apply to a namespace (example: `e-team`):

```bash
kubectl create namespace e-team --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f k8s/helm-rendered/e-team.yaml -n e-team
```

`helm template` uses `--set fullnameOverride=e-team` so resource names stay short (`e-team-api-gateway`, `e-team-postgres`, …).

## Helm (recommended for clusters)

Prefer `helm install` / `helm upgrade` — see [`charts/e-team/README.md`](../charts/e-team/README.md).

## Legacy layout

The previous `k8s/*.yaml` and `k8s/postgres/` hand-written files were removed in favor of the rendered chart output to avoid drift with Compose and Helm.
