# Alfred All-in-One K8s Lab

This directory scaffolds a near-Kubernetes test environment for Alfred inside a single Docker container.

## Goal

Provide one local environment that can exercise:

- Alfred in `serve` mode
- Redis-backed state
- Alertmanager webhook intake
- Prometheus query path
- Telegram delivery through a mock API
- read-only Kubernetes investigation flow inside `k3s`

## Current State

This is the initial scaffold.

It is designed for a privileged Docker container that runs:

- `k3s server`
- bootstrap scripts
- Kubernetes manifests under `manifests/`

## Planned Components

- `alfred`
- `redis`
- `mock-telegram-api`
- `sample-app`
- Prometheus and Alertmanager

## Layout

- `Dockerfile`: image for the all-in-one lab
- `entrypoint.sh`: boots `k3s` and applies manifests
- `manifests/`: Kubernetes resources applied into the lab cluster
- `scripts/`: helper scripts for bootstrap and scenario triggering

## Next Steps

1. Build the lab image.
2. Run it with Docker `--privileged`.
3. Verify `k3s` becomes ready.
4. Apply the initial manifests.
5. Add Prometheus and Alertmanager scenario automation.
