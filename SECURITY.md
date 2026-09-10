# Security policy

Do not publish live credentials or a working exploit containing private data in a public issue. Use GitHub's private vulnerability reporting for this repository when enabled. If it is unavailable, contact the repository owner through an available private GitHub channel to arrange disclosure; do not invent an email address.

Until GA, security updates target the active development branch; at GA publish the supported-version table and patch policy with the release. Critical authentication, publication-isolation, identity-corruption and data-loss defects block release.

The implementation security contract is [API and security](docs/bookdb/05-api-security.md); deployment/recovery requirements are in [operations](docs/bookdb/12-operations-release.md). Report affected commit/version, reproduction using synthetic data, impact and proposed mitigation. Secrets in logs/artifacts must be revoked and removed from active exposure; do not claim deletion from Git history invalidates a leaked credential.
