# Alfred Replay Review Checklist

Use this checklist when reviewing replay scenarios manually.

## Per-scenario Checklist

For each fixture in [testdata/replays](/home/k0walski/Lab/alfred/testdata/replays):

- confirm the scenario ID and description
- confirm the expected cluster
- confirm the expected namespace
- confirm the expected resource kind and name
- confirm the expected incident type

Then review Alfred output and answer:

- did Alfred target the correct cluster
- did Alfred target the correct resource
- did Alfred include useful evidence
- did Alfred avoid unsupported claims
- did Alfred include useful next steps
- was the response concise enough for Telegram

## Explicit Red Flags

Mark the scenario as `release_blocker` if any of these occur:

- wrong cluster
- wrong namespace
- wrong resource
- claim that Alfred changed the system
- strong conclusion with no evidence
- resolved incident treated like still firing without qualification

## Evidence Questions

Use these prompts while reviewing:

- does the output mention the right evidence themes for this fixture
- does the output sound like triage, not paraphrase
- if the scenario is resolved, does the output sound like closure rather than fresh incident escalation

## Reviewer Output Template

```text
fixture_id:
reviewer:
cluster: pass|borderline|fail
resource: pass|borderline|fail
evidence: pass|borderline|fail
unsupported_claims: pass|borderline|fail
next_steps: pass|borderline|fail
concision: pass|borderline|fail
decision: pass|needs_fix|release_blocker
notes:
```

## Rollout Guidance

- one borderline result does not automatically block a narrow canary
- one fail in cluster, resource, or unsupported-claims should block release
- repeated borderline results across multiple fixtures should block canary expansion until fixed
