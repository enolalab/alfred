# Alfred Release Quality Gate

Use this document before expanding Alfred beyond a narrow production canary.

## Goal

Make release decisions depend on both:

- runtime health
- output quality

Runtime health alone is not enough for Alfred. A stable but misleading incident investigator is still unsafe.

## Required Inputs

Before applying this gate, the following must already be true:

- deployment and rollback runbooks exist
- post-deploy smoke checks passed
- canary runtime health is acceptable
- replay fixtures exist in [testdata/replays](/home/k0walski/Lab/alfred/testdata/replays)
- reviewers use:
  - [evaluation-rubric.md](/home/k0walski/Lab/alfred/docs/evaluation-rubric.md)
  - [replay-review-checklist.md](/home/k0walski/Lab/alfred/docs/replay-review-checklist.md)

## Minimum Replay Gate

Before broader rollout, the team must review the current replay set and confirm:

- no fixture is marked `release_blocker`
- no fixture fails `correct cluster`
- no fixture fails `correct resource`
- no fixture fails `no unsupported claims`
- no fixture shows clear regression in Telegram concision
- no fixture contains phrasing that implies Alfred acted on the cluster

## Borderline Rule

Borderline results are allowed only if:

- they do not affect `correct cluster`
- they do not affect `correct resource`
- they do not affect `no unsupported claims`
- they are documented with notes and accepted by the reviewers

Repeated borderline outcomes across multiple fixtures should block canary expansion until fixed.

## Suggested Review Command

Generate the review scaffolding with:

```bash
go run ./cmd/alfred replay
```

Then review each scenario using the replay checklist template.

## Local Baseline Script

To run the current automated baseline locally:

```bash
bash scripts/release_quality_gate.sh
```

This script currently enforces:

- `go test ./...`
- replay scaffolding generation
- runtime Kustomize overlay rendering
- monitoring Kustomize overlay rendering

It does not replace human replay review, but it makes the baseline reproducible in CI.

## CI and Release Enforcement

The repo now includes:

- baseline CI workflow:
  - [ci.yml](/home/k0walski/Lab/alfred/.github/workflows/ci.yml)
- release readiness workflow:
  - [release-readiness.yml](/home/k0walski/Lab/alfred/.github/workflows/release-readiness.yml)
- sign-off validator:
  - [validate_release_signoff.sh](/home/k0walski/Lab/alfred/scripts/validate_release_signoff.sh)
- sign-off template:
  - [release-signoff.template.md](/home/k0walski/Lab/alfred/docs/release-signoff.template.md)

The baseline CI uploads the generated replay review and rendered manifests as artifacts.

The release readiness workflow fails unless:

- the baseline quality gate passes
- the provided sign-off file exists
- the sign-off file lives under `docs/release-signoffs/`
- the sign-off file has all required fields
- placeholder values such as `replace-me` are gone
- `decision: pass`

Governance guidance for sign-off ownership and branch protection lives in:

- [release-signoff-governance.md](/home/k0walski/Lab/alfred/docs/release-signoff-governance.md)

## Release Decision Outcomes

### `pass`

Use when:

- all replay scenarios pass the blocking dimensions
- no review shows obvious read-only trust-model violations
- no major quality concern is found

Result:

- canary may continue or expand, subject to runtime health

### `needs_fix`

Use when:

- there are borderline quality issues
- quality is usable for a narrow canary but not strong enough for broader rollout

Result:

- hold expansion
- fix and re-run replay review

### `release_blocker`

Use when:

- any replay fails `correct cluster`
- any replay fails `correct resource`
- any replay fails `no unsupported claims`
- any replay clearly implies Alfred restarted, rolled back, scaled, fixed, or changed the cluster

Result:

- do not expand canary
- fix before next release decision

## Recommended Sign-off

Capture:

- image tag under review
- fixture set version or commit SHA
- reviewer names
- replay outcomes
- final release decision

## Week 6 Behavior Baseline

The current quality gate assumes Alfred should now satisfy these behavior constraints:

- concise Telegram-oriented structure
- explicit read-only phrasing
- separation of observed evidence vs inferred causes
- cautious wording when evidence is weak
- no first-person action claims about changing cluster state
