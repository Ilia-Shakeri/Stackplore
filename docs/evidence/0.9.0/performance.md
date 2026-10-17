# 0.9.0 PostgreSQL page measurements

Measured on 2026-10-17 against disposable local PostgreSQL 17.11 on Windows,
with the repository's pinned Go 1.26.5. This is SQL and repository evidence.
It does not certify production capacity, HTTP throughput, browser rendering,
provider latency, cold storage, or a multi-source deployment.

## Reproduction and data

Prepare the pinned tools as described in [testing](../../testing.md), then set
`STACKPLORE_TEST_POSTGRES_URL` through a protected local environment. The URL
must name a disposable database permitted by the integration fixture gate.
Never print it or put it into saved command output.

```powershell
go -C apps/api test ./internal/postgres -run '^TestReleaseRepresentativeKeysetPlansOnRealPostgreSQL$' -count=1 -v
```

The test owns an isolated schema, applies the current migrations, inserts
synthetic rows, and removes that schema on cleanup. It adds 1,000 resources,
snapshots, events and resolved alerts, then grows each set to 20,000 rows.
Three existing resources/events and one existing firing alert remain alongside
the added rows. All data has one source and a published complete observation.
The default name-ordered inventory query, source event query and source alert
query are tested at the start, halfway through, and 100 rows before the end.
All query values are repository-controlled fixtures.

Every query uses `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` six times: one warmup,
then five measured samples. The test emits only row counts, timing, block
counts, sort counts and index names. It does not emit SQL values, cursor
contents, connection settings, or infrastructure details. Separate repository
calls verify 50 results, a continuation cursor, and the correct side of each
anchor. SQL plans return at most the 51-row lookahead.

## Measured correction

The first correct-data run exposed two query-path gaps:

- The alert feed's existing index led with state, whereas the feed orders all
  states together. At 20,000 rows its plan scanned and sorted the source.
- The inventory cursor disjunction let the planner scan earlier names and
  filter them before the page. At depth 19,900 it rejected 19,899 earlier rows.

Migration `000016_query_indexes.sql` adds the alert source/update-time/ID order
index. Inventory adds an explicit inclusive primary cursor bound before its
existing disjunction; alerts likewise bound update time explicitly. These
predicates preserve equal-value opaque-ID order and the existing ascending,
descending and `C`-collation rules. No cursor format changes.

The pre-correction measured run passed its bounded-page correctness checks in
27.38 seconds. The following medians are from its five warm samples:

| Added rows | Cursor depth | Inventory ms | Events ms | Alerts ms |
| ---------- | ------------ | ------------ | --------- | --------- |
| 1,000      | 0            | 0.995        | 0.203     | 1.186     |
| 1,000      | 500          | 1.174        | 0.194     | 0.737     |
| 1,000      | 900          | 1.960        | 0.313     | 0.440     |
| 20,000     | 0            | 1.276        | 0.204     | 82.827    |
| 20,000     | 10,000       | 25.027       | 0.190     | 24.252    |
| 20,000     | 19,900       | 38.534       | 0.288     | 18.307    |

The before run's 20,000-row alert first-page samples ranged from 51.008 to
128.580 ms. Concurrent workstation work makes these figures variable. Its
plans touched 626 shared blocks and used no alert index. The deep inventory
plan touched 864 shared blocks. All reported plans used zero temporary blocks
and zero physical shared reads after warmup. These are observations, not
latency acceptance thresholds.

The final post-correction real database run passed in 6.93 seconds. Every plan
returned 51 lookahead rows and all repository calls returned 50 rows with
continuation. Its five-sample medians were:

| Added rows | Cursor depth | Inventory ms | Events ms | Alerts ms |
| ---------- | ------------ | ------------ | --------- | --------- |
| 1,000      | 0            | 0.340        | 0.068     | 0.037     |
| 1,000      | 500          | 0.368        | 0.067     | 0.045     |
| 1,000      | 900          | 0.636        | 0.105     | 0.087     |
| 20,000     | 0            | 0.765        | 0.120     | 0.079     |
| 20,000     | 10,000       | 0.862        | 0.181     | 0.085     |
| 20,000     | 19,900       | 1.024        | 0.209     | 0.086     |

At 20,000 rows the alert plans used `alerts_source_order_idx`, touched 4–6
shared blocks and needed no sort. Inventory used
`resource_snapshots_name_idx`, touched 268–272 shared blocks and rejected no
earlier-name rows. Its remaining sort resolves the opaque-ID tie order for
the small indexed candidate set. The deepest event query retained its source
order index. All final measured plans used zero temporary blocks and physical
shared reads. These plan changes substantiate the correction without treating
the timing difference as an isolated hardware-controlled speedup experiment.

The regression also requires the relevant inventory and alert indexes at
20,000 rows and rejects scans that filter more than one lookahead page of
earlier-anchor rows for this fixture. It imposes no fixed millisecond gate.

The final alert query/index explicitly use C collation for opaque IDs. A separate real PostgreSQL regression paginates mixed-case IDs at the same timestamp without omissions, repeats or locale ordering; it passed in 1.34 seconds. The migration, all inventory sort/direction combinations, maximum Unicode name, immutable renamed anchor and cursor tamper regressions also passed after the range-bound changes.

## Limits and failed fixture receipt

This fixture has unique inventory display names and uniform event/alert times.
Repeated names/timestamps, many sources, compound filters, partition denial,
write contention, retention pressure and cold caches need their own workloads.
Page bounds alone do not prove constant-time work or a deployment's capacity.
The test asserts query structure and page correctness rather than a fixed
millisecond threshold.

An initial fixture attempt failed because its inserted observations were newer
than the source's observation receipt. The repository correctly rejected that
page. The fixture now uses the trusted completion clock. That failure is not
counted as a successful product check or as a latency measurement. Local
receipts are `.tmp/stage8-performance-fixture-failure.txt`,
`.tmp/stage8-performance-before.log`, `.tmp/stage8-performance-after.log`, `.tmp/stage8-performance-final.log` and `.tmp/stage8-performance-canonical-final.log`;
they are ignored working files. This document preserves their sanitized
method, observations and limitations.
