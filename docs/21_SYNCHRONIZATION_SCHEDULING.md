# Source Synchronization and Scheduling

## Owner requirement

BookDB may use continuous Internet access for normal operation, but it must **not constantly poll all providers**.

## Trigger types

A connector can run from:
1. scheduled synchronization;
2. Administrator `Sync Now`;
3. upstream webhook/event where supported and allowed;
4. bootstrap import;
5. explicit recovery/replay.

No connector has an uncontrolled while-loop polling mode.

## Schedule model

Each source configuration includes:

```yaml
enabled: true
mode: incremental
schedule:
  type: cron
  expression: "17 3 * * *"
  timezone: UTC
  jitter_seconds: 900
maintenance_window:
  max_runtime: 2h
limits:
  max_concurrency: 4
  requests_per_second: 1
  daily_request_budget: 50000
retry:
  max_attempts: 6
  backoff: exponential
```

## Suggested default classes

### Bulk-dump sources
Check for a new dump daily or weekly using inexpensive metadata/conditional requests; ingest only when version/checksum changed.

### Incremental change feeds
Run at a source-appropriate cadence, typically hourly/daily, based on documented provider expectations.

### Slow/static authority sources
Daily/weekly.

### Asset refresh
Only when:
- canonical asset missing;
- source asset changed;
- scheduled staleness policy marks asset due.

### Manual
Administrator can:
- full sync;
- incremental sync;
- entity/source-ID refresh;
- retry failed batch;
- replay source snapshot.

## Conditional network behavior

Use where allowed:
- ETag;
- If-Modified-Since;
- source revision IDs;
- cursor/checkpoint tokens;
- dump checksums/manifests.

## Checkpoints

A source sync stores:
- sync execution ID;
- source policy version;
- connector version;
- upstream cursor/version;
- start/end;
- counts;
- checksum;
- failure/quarantine counts.

Checkpoints are committed only at safe boundaries.

## Backpressure

If downstream reconciliation/indexing is overloaded:
- ingestion pauses or reduces concurrency;
- scheduler does not stack unlimited duplicate runs;
- new runs coalesce into one pending request where semantics permit.

## Admin UX

For every source show:
- next scheduled run;
- last successful run;
- last attempted run;
- current checkpoint;
- source version;
- policy status;
- queue depth;
- error rate;
- Sync Now button;
- Pause button.
