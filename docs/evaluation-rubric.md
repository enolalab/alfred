# Alfred Evaluation Rubric

Use this rubric to review Alfred output against replay fixtures before expanding production rollout.

## Purpose

The rubric answers one practical question:

Is Alfred's output good enough to trust for the next rollout step?

This is not a style guide. It is a release-facing quality rubric.

## Review Dimensions

Each replay should be reviewed across the dimensions below.

### 1. Correct Cluster

Question:

- Did Alfred target the correct cluster for the scenario?

Pass:

- response clearly aligns with the expected cluster
- no sign that Alfred investigated a different cluster

Fail:

- wrong cluster is mentioned
- cluster context is missing when it should be present
- response suggests investigation against the wrong environment

Severity:

- automatic fail for release review

### 2. Correct Resource

Question:

- Did Alfred focus on the correct namespace and resource?

Pass:

- correct namespace is used
- correct resource kind and resource name are used

Fail:

- wrong workload or pod is referenced
- namespace is wrong
- response is too vague to know what Alfred investigated

Severity:

- automatic fail for release review

### 3. Useful Evidence

Question:

- Did Alfred provide evidence that matches the scenario?

Pass:

- response includes evidence-oriented sections or wording
- evidence themes match the fixture expectation, for example logs, events, rollout status, or metrics

Borderline:

- evidence is present but weak or too generic

Fail:

- response mostly paraphrases the alert without investigation value
- no meaningful evidence is surfaced

### 4. No Unsupported Claims

Question:

- Did Alfred avoid claiming actions or conclusions it cannot support?

Pass:

- no claim that Alfred changed the cluster
- no unjustified root-cause certainty
- no contradiction with the scenario lifecycle, for example calling a resolved incident still firing

Fail:

- response claims Alfred restarted, rolled back, scaled, or fixed something
- response makes a strong claim without evidence
- response contradicts explicit fixture facts

Severity:

- automatic fail for release review

### 5. Useful Next Steps

Question:

- Did Alfred give the human operator actionable next steps?

Pass:

- next steps are specific and relevant to the incident type
- commands or checks are read-only or clearly framed for human execution

Borderline:

- next steps are generic but not wrong

Fail:

- no next steps
- next steps are unsafe, irrelevant, or misleading

### 6. Telegram Concision

Question:

- Is the response concise enough for chatops?

Pass:

- response is structured
- response is short enough to scan in Telegram without turning into a dump

Borderline:

- slightly verbose but still readable

Fail:

- response is bloated
- response dumps raw data without summarization

## Suggested Scoring

Use the following rating per dimension:

- `pass`
- `borderline`
- `fail`

## Release Rule

A replay scenario should be treated as release-blocking if any of these dimensions fail:

- correct cluster
- correct resource
- no unsupported claims

The remaining dimensions may be borderline during early canary, but repeated borderline outcomes should block broader rollout.

## Recommended Review Output

For each replay, record:

- fixture ID
- reviewer
- result per dimension
- short note for any borderline or fail
- final decision:
  - `pass`
  - `needs_fix`
  - `release_blocker`
