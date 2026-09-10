# WebUI product and design specification

Status: target user experience. A modern UI is measured by usable catalog workflows, coherent visual hierarchy, accessibility and interaction quality—not by a dashboard full of fabricated statistics.

## Information architecture

| Area | Main screens | Key actions |
| --- | --- | --- |
| Discover | Search, browse, results | Search title/author/ISBN; filter by format, language, series and contributor role; switch grid/list; sort; retain state in URL |
| Catalog | Work, expression, edition, person, organization, series | Follow relationships; compare editions; inspect contributor credits and evidence; propose corrections |
| Account | Profile, API keys, own proposals | Create/revoke keys, view quotas, review proposal status |
| Administration | Review queue, comparison/approval, merge/split | Review evidence side by side; inspect conflicts and affected identities; approve with a reason |
| Sources | Source list/detail, schedules, jobs, quarantine | Configure/pause/trigger; see progress, freshness, errors and recovery actions |
| Quality and operations | Coverage, conflicts, duplicate candidates, service status | Drill into affected records and jobs; distinguish missing coverage from source failure |

Avoid generic development “workspace” pages in the production navigation. The inspected React shell can be reused structurally, but user-facing labels and content must describe books and catalog administration. Show only actual metrics; an empty installation should explain how to ingest its first catalog.

## Core journeys and acceptance

1. **Find the right publication.** Search a title, narrow by language and audio/ebook, open a work, compare editions, inspect narrator/publisher/date/abridgment, copy its BookDB ID or API example. Unknown fields are visibly unknown. A default format filter must not hide other formats unexpectedly.
2. **Understand a contributor.** Open an author, translator, narrator or publisher; see aliases, credits grouped by role, works/performances and supported biographical detail. Do not label everyone “author.”
3. **Correct a record.** Choose a field, provide evidence, preview the proposed diff and submit. The existing public page remains unchanged; the contributor can inspect status and administrator feedback.
4. **Review a proposal.** Filter pending items, compare canonical/current/proposed values and source evidence, detect stale versions, approve/reject with a reason, then verify published results. Keyboard operation must be practical.
5. **Operate ingestion.** Select source, preview schedule and next run, manually trigger a bounded job, inspect counts and quarantine reasons, pause/resume without losing progress.
6. **Use the API.** Create a scoped key, copy it once, see a usable request example, inspect usage/expiry, rotate and revoke. Never redisplay stored secrets.

## Visual system

Use a restrained editorial/catalog style: clear typography, generous but purposeful spacing, book-cover imagery when permitted, readable tables, compact badges for formats/languages and clear field groupings. Build design tokens for color, spacing, type, radii, elevation, motion and focus states. Support light/dark modes and system preference; verify contrast in both. Choose free fonts or system fonts and retain licenses for any bundled assets.

Keep discovery uncluttered; advanced detail belongs in expandable field groups or dedicated tabs. On work pages, give the title, contributors, synopsis and publication choices priority. On edition pages, emphasize facts distinguishing that edition. Admin screens may be dense but must show the selected record, proposed changes and evidence together without forcing repeated navigation.

Do not install an entire design/agent plugin collection. A small accessible component system, React tooling and Playwright are sufficient foundations. Before an optional UI package is added, verify that required widgets do not sit behind a commercial tier. Component choices are implementation decisions recorded once, not new product scope.

## Interaction requirements

| Element | Required states |
| --- | --- |
| Search | Loading, results, no results, error, partial/stale projection; cancellable/debounced request; browser back restores state |
| Tables | Stable row keys, server pagination, accessible sorting, clearly scoped selection, overflow handling |
| Forms | Labels, field validation, summary errors, unsaved-change handling, disabled submission while pending |
| Mutation | Pending, success, validation conflict, authorization failure and recoverable network failure; no duplicate submit |
| Assets | Fixed-aspect placeholders, alt text, unknown/missing/blocked distinction when useful, lazy loading |
| Long jobs | Progress stage/counts, last update, cancel/pause semantics; do not invent percentages with unknown totals |
| Proposals | Private/pending/approved/rejected/needs changes; diff and rationale accessible without color alone |

Initial responsive acceptance widths: 360 px, 768 px and 1440 px. No essential controls outside viewport; tables may use intentional horizontal scrolling with labels preserved. Support keyboard focus order, visible focus, escape/return behavior, screen-reader labels, reduced motion and 200% text zoom. Automated accessibility tools do not replace human keyboard/screen-reader testing.

## UI delivery process

For each screen slice: state the user journey; sketch layout in the issue if necessary; implement with real API data; test loading/empty/error/permission states; capture screenshots at the three widths; run accessibility checks; run the written human test at the stage gate. A screenshot alone is insufficient evidence of working data or authorization.

At S2, deliver functional search/results/detail with real imported records and minimal local-admin session access. At S4, complete account/OIDC/moderation/key management. At S5, complete edition comparison, contributors/series, rich filtering, admin jobs/quality, responsive design and accessibility. The interface should improve alongside behavior; do not postpone all UI until every backend service is finished.
