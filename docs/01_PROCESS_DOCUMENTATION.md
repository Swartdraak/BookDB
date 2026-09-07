# Process Documentation

Lifecycle:
1. issue intake;
2. triage;
3. design/ADR where required;
4. implementation;
5. tests;
6. review;
7. merge;
8. release;
9. observation;
10. retrospective.

ADR required for:
- canonical entity semantics;
- schema architecture;
- source legal status;
- public API break;
- auth model;
- persistence/search technology.

Public API changes require OpenAPI diff and contract tests.
Schema changes require migration and representative-scale test.
Source connectors require source-policy approval and fixtures.
Reconciliation changes require before/after gold-corpus metrics.
Security-sensitive changes require security-agent review.
