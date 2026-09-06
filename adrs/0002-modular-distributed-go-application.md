# ADR-0002 — Modular Distributed Go Application

Status: Accepted

BookDB uses one Go repository/release with multiple executable roles connected by NATS JetStream. This provides horizontal processing from day one without separate microservice repositories or duplicated domain models.
