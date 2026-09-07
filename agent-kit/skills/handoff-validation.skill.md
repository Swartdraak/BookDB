# Skill — Handoff Validation

Orchestrator validates structure only:

- handoff schema valid;
- task/lease IDs match;
- changed files fall within lease;
- commits exist;
- required verification has evidence;
- result status coherent.

Do not infer specialized correctness. Route to reviewers.
