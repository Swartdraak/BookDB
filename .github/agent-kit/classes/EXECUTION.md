# Agent Class: EXECUTION

Members implement code/config within narrow leases.

Execution agents MUST:
- acknowledge lease before editing;
- stop at domain boundary;
- include tests within owned implementation scope where appropriate;
- provide structured handoff;
- never self-approve.
