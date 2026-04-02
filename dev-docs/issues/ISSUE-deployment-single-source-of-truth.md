# Deployment: single source of truth (Helm chart + generated artifacts)

## Problem

The repository currently supports Docker Compose, hand-written Kubernetes manifests, and Helm. Without a canonical source, image tags, database credentials, and service wiring can drift between paths.

## Proposal

1. **Canonical source**: `charts/e-team/values.yaml` remains the single place to change versions, images, and DB auth defaults for the stack.
2. **Docker Compose**: Load runtime image references and Postgres credentials from a generated `compose.values.env` file produced by `scripts/gen-compose-env-from-values.rb` (values-driven). Use `docker compose --env-file compose.values.env ...` so interpolation matches the chart defaults.
3. **Raw Kubernetes**: Stop maintaining duplicate YAML under `k8s/`. Replace with **rendered** manifests from `helm template`, checked in under `k8s/helm-rendered/` with `scripts/render-k8s-manifests.sh`. Document that edits go through the chart (or values overlays), then re-render.
4. **Verification**: Add lightweight scripts to regenerate and optionally diff generated files in CI or pre-commit.

## Non-goals (for this iteration)

- Removing Docker Compose (still the primary local-dev path).
- Adding RabbitMQ to the Helm chart (Compose-only dependency today).

## Acceptance criteria

- [ ] `compose.values.env` is generated from `values.yaml` and documented.
- [ ] `k8s/helm-rendered/` reflects `helm template` output; legacy hand-written manifests are removed or clearly deprecated.
- [ ] README / dev-docs updated with the new workflow.
