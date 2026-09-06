# Skill — HA Failure Test

1. Name failure domain.
2. State expected behavior/RPO/RTO.
3. Capture healthy baseline.
4. Fail one component intentionally.
5. Verify quorum/failover/fencing.
6. Verify application behavior.
7. Rejoin component.
8. Verify redundancy restored.
9. Record evidence.

Multiple replicas on one physical host do not prove hardware HA.
