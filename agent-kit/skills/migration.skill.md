# Skill — Database Migration

1. Confirm domain semantics frozen.
2. Confirm released migrations are not being edited.
3. Use forward migration.
4. Add constraints/indexes intentionally.
5. Test fresh DB.
6. Test upgrade from supported prior state.
7. Benchmark representative volume if large table.
8. Document lock/rewrite/failover implications.
9. Submit for QA and HA review as required.
