# Workflow: Source Synchronization

schedule/manual/event -> deduplicated sync execution -> connector -> checkpointed source records -> claims -> reconciliation -> canonical outbox -> search/change feed.
