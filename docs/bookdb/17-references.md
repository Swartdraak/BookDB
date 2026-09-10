# Research and evidence register

Checked: 2026-09-10. Sources below are official documentation or primary project repositories. Links are reference evidence, not instructions granting write permission. Version-sensitive facts must be rechecked when used. Bibliographic architecture, stage structure, quota/performance targets and governance choices are **BookDB design decisions**, not claims copied from these sources.

The repository audit records pinned BookDB/Bibliophilarr evidence separately. Chat history was available through retrieval and summaries, not a guaranteed export of every historical message. Current explicit owner answers supersede older project choices where they differ.

| ID | Primary source | Use and qualification |
| --- | --- | --- |
| R01 | [JetBrains DataGrip MCP](https://www.jetbrains.com/help/datagrip/mcp-server.html) | Integrated MCP setup, tool exposure and database-specific capabilities/conditions. Inspect installed capabilities; some convenience tools have plan limits. |
| R02 | [JetBrains WebStorm MCP](https://www.jetbrains.com/help/webstorm/mcp-server.html) | IDE-generated connection configuration and exposed tools; use actual endpoint/project settings. |
| R03 | [GitHub customization support](https://docs.github.com/en/copilot/reference/customization-cheat-sheet) | JetBrains instructions/customization/MCP support and current hook support limitations. Cross-check repository instructions: https://docs.github.com/en/copilot/reference/custom-instructions-support . |
| R04 | [GitHub MCP setup for IDEs](https://docs.github.com/en/copilot/how-tos/provide-context/use-mcp-in-your-ide/set-up-the-github-mcp-server) | JetBrains remote OAuth configuration uses servers/github and the official MCP URL. Permissions remain account-scoped. |
| R05 | [Microsoft Playwright MCP](https://github.com/microsoft/playwright-mcp) | Open-source browser automation server using structured page/accessibility information; pin an installed version. |
| R06 | [Open Library dumps](https://openlibrary.org/developers/dumps) | Monthly bibliographic dump records have typed keys/revisions/timestamps/JSON. Snapshot import is the bulk path; covers have separate availability. |
| R07 | [Open Library licensing statement](https://openlibrary.org/developers/licensing) | Internet Archive does not assert new proprietary rights and notes possible existing rights. Do not restate this as blanket CC0 for every field/asset. |
| R08 | [Open Library API guidance](https://openlibrary.org/developers/api) | Low-volume usage, identification and limits; bulk users are directed to dumps rather than API harvesting. |
| R09 | [Wikidata downloads](https://www.wikidata.org/wiki/Wikidata:Database_download) | Structured entity data is CC0; other text/media can have different licenses. |
| R10 | [DOAB metadata](https://www.doabooks.org/en/article/metadata) | Official search retrieval describes metadata feeds as CC0. Direct open failed in this environment; verify live interface at source onboarding. Corroborating official FAQ: https://www.doabooks.org/en/researchers/full-faq . |
| R11 | [LibriVox API](https://librivox.org/api/info) | Official search retrieval confirms API endpoints. Direct open failed; exact metadata/asset reuse conditions were not established here. Candidate, not approved enabled source. |
| R12 | [GitHub custom agent configuration](https://docs.github.com/en/copilot/reference/custom-agents-configuration) | Agent profile properties/tool aliases, model inheritance and IDE-specific differences. Unrecognized tool names may be ignored; confirm actual permissions. |
| R13 | [Readarr retirement notice](https://github.com/Readarr/Readarr) | The project attributes retirement partly to unusable metadata and a stalled source transition. Supports metadata-continuity motivation, not an assertion that BookDB solves it already. |
| R14 | [IFLA Library Reference Model](https://repository.ifla.org/handle/123456789/40) | Conceptual work/expression/manifestation/agent framework. BookDB makes its own pragmatic schema decisions; no full standards-conformance claim. |
| R15 | [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions) | Standard public-repository workflows have a free usage path; storage and other runner classes require current billing review. |
| R16 | [Projects GraphQL API](https://docs.github.com/en/issues/planning-and-tracking-with-projects/automating-your-project/using-the-api-to-manage-projects) | Projects automation uses account-authorized GraphQL operations/scopes; do not assume repository token coverage. |
| R17 | [GitHub CLI Projects](https://cli.github.com/manual/gh_project) | Native commands for Project create/link/fields/items; exact commands use current installed CLI support. |
| R18 | [NATS JetStream delivery](https://github.com/nats-io/nats.docs/blob/master/nats-concepts/jetstream/README.md) | At-least-once failure cases motivate durable application idempotency and explicit transaction boundaries. |
| R19 | [OWASP REST security](https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html) | Credentials do not belong in URLs; use authenticated/authorized bounded operations. BookDB defaults are project decisions. |
| R20 | [OWASP API resource consumption](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/) | Rate limits alone are insufficient without bounded payloads, concurrency and expensive operations. |
| R21 | [GitHub Wiki editing](https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages) | Wikis use a distinct .wiki.git repository; initial UI creation may be required before Git access. |
| R22 | [Bookshelf reference](https://github.com/pennydreadful/bookshelf) | README distinguishes Goodreads/Hardcover metadata behavior and compatibility; chosen reference fork, not owner-confirmed exact fork. |
| R23 | [Official GitHub MCP server](https://github.com/github/github-mcp-server) | Open-source local server and remote integration; use minimal relevant tools instead of building a custom management server. |
| R24 | [GitHub March 2026 JetBrains updates](https://github.blog/changelog/2026-03-11-major-agentic-capabilities-improvements-in-github-copilot-for-jetbrains-ides/) | Documents MCP auto-approval configuration and agentic improvements. Installed capability probes remain required. |

## Interpretation limits

No vendor GPU/model compatibility or local inference concurrency benchmark was performed. The model/endpoint/hardware are owner-supplied constraints. No API key, local-model configuration, private network address or account entitlement is invented. No source is declared approved solely because it offers an API, and no complete global book catalog is promised from the current open sources.

Source definitions/terms can change. During connector implementation validate the exact dataset, supported interface, redistribution scope, attribution and asset rules. Record the checked URL/date in that source policy without turning every source update into a repository-wide governance project.
