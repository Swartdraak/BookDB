# Authentication, OIDC, Local Accounts, and Authorization

## Supported authentication

BookDB supports both:
- local accounts;
- OIDC federation.

Examples of compatible OIDC providers include Authentik and Keycloak, but BookDB implements standards-based OIDC rather than vendor-specific authentication.

## Local accounts

Requirements:
- Argon2id password hashing;
- password reset through secure administrator flow and/or configured email flow;
- optional future TOTP/WebAuthn;
- session revocation;
- failed-login throttling;
- account disable/lock;
- recovery codes if local MFA is later enabled.

## OIDC

Configuration:
- issuer URL;
- client ID;
- client secret reference;
- redirect URI;
- scopes;
- claim mappings;
- role/group mappings;
- optional auto-provision;
- optional required group.

Security:
- Authorization Code + PKCE where applicable;
- validate issuer/audience/nonce/state;
- key rotation/JWKS;
- no trust in unverified role headers from reverse proxies.

## Account linking

A local and OIDC identity may be linked only through authenticated account-management/admin workflow. Email equality alone must not automatically merge identities unless explicit instance policy permits it.

## Bootstrap/break-glass

Recommended:
- initial local Administrator created during first run;
- OIDC configured later;
- operator may retain one disabled-unless-needed emergency local Administrator account;
- emergency use is audit logged.

## RBAC

Permissions, not only named roles, are canonical.

Example permissions:
- catalog.read
- provenance.read
- proposal.create
- proposal.triage
- proposal.admin_approve
- entity.merge
- entity.split
- source.read
- source.configure
- source.sync
- jobs.manage
- users.manage
- api_clients.manage
- system.maintenance
- audit.read

`proposal.admin_approve` is granted only to Administrator-level roles.

## API authentication

Machine clients use:
- hashed API keys with scopes;
- expiration optional/encouraged;
- last-used metadata;
- per-key rate policy;
- immediate revoke.

Future OAuth client credentials can be added without changing the native API resource model.
