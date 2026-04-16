# Alfred Kustomize Layout

This directory provides a baseline Kustomize structure for Alfred runtime deployment.

Structure:

- `base/`
  - runtime core manifests only
  - service account, RBAC, deployment, service
- `overlays/staging/`
  - uses `configs/config.staging.example.yml`
  - targets namespace `alfred-staging`
- `overlays/production/`
  - uses `configs/config.production.example.yml`
  - targets namespace `alfred`
- `overlays/production-external-secrets/`
  - extends `production/`
  - adds an `ExternalSecret` resource for `alfred-secrets`

Notes:

- Monitoring manifests remain under `deploy/monitoring/` because Prometheus Operator resources often live in a separate namespace such as `monitoring`.
- The overlays generate `alfred-config` from the repo config examples. Replace these example configs with environment-specific files before real deployment.
- Each overlay keeps its own `config.yml` so `kustomize build` works with the default load restrictor. Replace these files with environment-specific runtime config before real deployment.
- The staging overlay generates `alfred-secrets` from `secret.env` for local or non-production use.
- The production overlay does not generate secrets by default. Use `externalsecret.example.yaml` as the preferred production pattern, or adapt `secret.env.example` only for manual/non-production workflows.
- The `ClusterRoleBinding` subject namespace is patched per overlay so the service account binding stays correct.

Recommended secret path:

- staging or local clusters:
  - edit `overlays/staging/secret.env`
  - build/apply the staging overlay
- production clusters:
  - create a real `ExternalSecret` from `overlays/production/externalsecret.example.yaml`
  - keep credentials out of Git and out of generated manifests
  - or use `overlays/production-external-secrets/` as the Kustomize entrypoint after replacing placeholder secret-store values

Files to review for secrets:

- `overlays/staging/secret.env`
- `overlays/production/externalsecret.example.yaml`
- `overlays/production/secret.env.example`
- `overlays/production-external-secrets/externalsecret.yaml`

Examples:

- `kustomize build deploy/kustomize/overlays/staging`
- `kustomize build deploy/kustomize/overlays/production`
- `kustomize build deploy/kustomize/overlays/production-external-secrets`
