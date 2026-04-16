# Release Sign-off Governance

Use this document to turn the current release-readiness workflow into an enforceable release control.

## Goal

Ensure a release sign-off is:

- written by real reviewers
- reviewed by the correct owners
- tied to a concrete release decision

## Minimum Policy

Apply these repository rules:

1. Require release sign-off files to live only under:

- `docs/release-signoffs/`

2. Protect the default branch so changes to release sign-off files require review.

3. Require the `Release Readiness` workflow to pass before any production release is approved.

## Recommended CODEOWNERS Scope

If your GitHub organization uses CODEOWNERS, add a rule for:

- `docs/release-signoffs/*`

Suggested owner groups:

- platform owners
- production on-call owners
- release managers

Do not use a placeholder owner in the real CODEOWNERS file. Wire it to actual team handles in your GitHub organization.

A repo template is available at:

- [.github/CODEOWNERS.template](/home/k0walski/Lab/alfred/.github/CODEOWNERS.template)

## Recommended Branch Protection

For branches that can lead to production rollout:

- require pull request review before merge
- require status checks:
  - `CI`
  - `Release Readiness` when applicable
- require review from code owners for `docs/release-signoffs/*`

## Recommended Release Process

1. Run baseline CI.
2. Generate replay review artifacts.
3. Create a real sign-off file in `docs/release-signoffs/`.
   Do not use `docs/release-signoffs/README.md`, `docs/release-signoff.template.md`, or a placeholder filename such as `replace-with-real-signoff.md`.
4. Have the sign-off reviewed by the designated owners.
5. Run `Release Readiness` with that sign-off path.
6. Proceed only if the workflow passes and the decision remains `pass`.

## Anti-patterns

Do not allow:

- using `docs/release-signoff.template.md` as a real sign-off
- placeholder values such as `replace-me`
- unsigned or unreviewed sign-off files
- release decisions made outside the repository record
