# ADR-0003 — PostgreSQL 18 + Patroni Reference HA

Status: Accepted

PostgreSQL remains canonical. Reference HA uses streaming replication, Patroni and etcd, fronted by PgBouncer/load-balanced endpoints. BookDB supports externally managed PostgreSQL HA implementations that honor connection/failover contracts.
