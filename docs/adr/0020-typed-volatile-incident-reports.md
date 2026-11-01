# ADR-0020: Typed volatile incident reports

Status: accepted design; stage-6 source and quality acceptance remain pending.
Date: 2026-11-01.

## Context

The accepted bounded investigation has a fixed read plan, distinct sealed proofs,
known usage and one final generation. Incident chronology must distinguish actual
event/sample time from current observation time. Provider prose and generated
instructions cannot introduce a causal claim, executable recommendation or new
read authority. Alert/replica history requires separate narrow ports and proofs.

## Decision

Project scalar observations from bounded context bundles without prose, log
messages, native handles or arbitrary reason codes. A pure correlation package
checks the selected identity/versions/window, preserves every actual reference,
sorts actual times with stable ties, and emits only closed qualified hypotheses.
Matching positive limits are required for pressure. Counter resets, simultaneous
conflicts, stale/partial/old data and unread families remain explicit gaps.
Ordering is not probability; correlation is not proof of cause.

Retain the report with the owning volatile run. Clone nested arrays for reads
and erase it on the same lifecycle fence as the answer. Use the existing fresh
proof checks before and after completion, on reads and on citations. Derivation
has no I/O and adds no tool, inference call, retry or persistent text.

Expose a separate explicit owning report read, without changing ordinary answer
receipts. Bind the report to exact fresh references and independently recompute
all claims/ranks/gaps/recommendations before wire conversion. Send only closed
typed scalar facts and inert next-read codes; no generated URL/action/command.
Report and answer share the existing 64-KiB canonical content budget. Pending,
lost and unavailable responses remain metadata only. New clients must validate
the closed shape, scope, timestamps, exact reference membership and qualifiers,
and discard results across owner/snapshot/lifetime changes.

## Verification and consequences

Run pure chronology, limit, counter-reset, conflict, insufficient-evidence,
mutation and independent-claim tests. Prove owner/current authorization, no late
content, no hidden output allowance, no text/log identity leakage and truthful
loss of volatile memory. Require generated contract parity and bilingual desktop
keyboard/RTL tests on Actions, then all eight source jobs and signed scans.
Held-out quality and human review remain independent prerequisites for stage-6
acceptance. Internal typed rules do not establish complete diagnosis quality.

Rejected alternatives: generation-selected tools; executable recommendations;
persisted report text; invented OOM/connectivity causes; treating source-wide
alert pages as resource authority; and an independent second payload allowance.
