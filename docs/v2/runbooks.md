# Smart Assistant — reviewed runbooks and metadata history

Development preview candidate: **2.0.0-alpha.6.3**, schema19. This metadata
increment does not claim completion of the reviewed-corpus/search stage.

Hosted preparation attempts: pure model source `2e78a91` passes seven jobs and
fails only Go formatting in run `41810618993`; its hosted formatting patch is
applied at `df275bd`. Owner-history source `72fcc59` reaches the actual SQL
regression in run `42454637270`; a second-page timestamp cannot use the parameter's
implicit integer type. The correction fixes that parameter to `bigint`, preserving
keyset limits and selected owner. New cursor/use-case checks require their own
hosted execution. No workstation test or formatter runs.

The next SQL attempt at `d4e7b96` in run `42455079592` passes keyset pagination
and owner isolation, then rejects an incomplete restore fixture: a positive
generation also requires its restore timestamp. The fixture supplies both,
preserving the real database constraint. Its hosted formatting patch is applied;
the failed run is not acceptance.

Status: **stage-7 preparation only**. The pure corpus model, owner-metadata read
port, authenticated HTTP boundary and typed client are authored on `v2/runbooks-prep`.
They create no database table and do not complete stage 7. Stage-6 quality review and
pilot resources remain unavailable; hosted preparation continues by instruction.

## Corpus identity and bounds

A candidate corpus has a canonical lowercase ID, positive version, explicit
audience (`administrators` or `all_users`) and at most 32 documents. Each document
has a canonical ID, English/Persian locale, title at most 256 UTF-8 bytes and body
at most 16 KiB. Aggregate candidate content is at most 512 KiB. A document/locale
pair is unique. Visible valid UTF-8 is required; line feed/tab are permitted in
body prose. Bidi/format controls are rejected except the joining characters needed
for natural Persian. Input order does not change identity. Exact bytes, document
identity, locale, title, version and audience are bound into a domain-separated,
length-framed SHA-256 digest. No text normalization happens silently after review.

Validation is structural. Hostile instructions can remain inert prose; the
validator does not claim to recognize all dangerous meaning. Ordinary formatting
and JSON serialization of candidate values conceal content. Explicit approved
transport/storage projections must be separate from these protected values.

Search input is at most 256 UTF-8 bytes and ten results. Snippets are at most 512
UTF-8 bytes, preserve complete code points and are rendered as plain text. This
preparation has no search engine or approval state. The corpus model performs no I/O.

## Owner-metadata repository preparation

The separate metadata port queries existing run metadata by authenticated owner,
one exact state, current restore generation and a seven-day retention window.
It selects safe columns only, at most ten entries plus one sentinel, ordered by
creation time and opaque ID descending. Paired keyset fields exclude the prior
row even when times tie. The existing owner/state/time index matches the required
filter; actual hot-path query plans still need measurement before performance claims.

The active current session is checked on both sides of the bounded transaction;
an old run's expired session does not need to be resurrected to read safe metadata.
An administrator has no other-owner shortcut. Changing restore generation hides
old records. No question, answer, report, evidence body, session digest or provider
credential is selected. The candidate HTTP boundary is registered only with the
enabled authenticated run service. The candidate desktop history view is a separate
lazy-loaded `/assistant/history` page linked from the Smart Assistant workspace.

The metadata use case adds a ten-minute authenticated cursor bound to owner,
session, authentication revision, state, page size, anchor and restore generation.
Malformed, expired or changed-scope cursors fail before repository I/O. Each read
has a three-second deadline and validates the returned shape, generation and
strict ordering before releasing a detached metadata page. It cannot admit a run,
replay text, widen the current actor or turn a stored reference into read authority.
Shutdown cancels reads and clears the cursor signing key. The candidate
`GET /api/v1/assistant/history` accepts only `state`, `limit` and `cursor`;
state defaults to `completed`, page size to ten. Duplicate, empty, unknown and
malformed arguments fail before repository access. Owner identity comes solely
from the current session. Responses are `no-store`, contain closed metadata rows
and a nullable next cursor, and have no session, restore-generation or text fields.
The typed client checks shape, ordering, timestamps, count bounds and cursor loops.
Contract derivation, race, SQL and negative HTTP/client tests run only on Actions.
These newly authored boundaries still need their own passing receipt;
reviewed-corpus approval and search remain open.

The desktop page replaces ten rows per page, uses a URL-backed closed status filter
and keeps at most 32 page cursors in volatile component memory. It clears displayed
rows/cursors on owner change or page exit, cancels outstanding reads, ignores late
old-owner replies and starts from the first page on return. Loading, empty and
unavailable/expired pages have distinct states. Explicit refresh resets pagination;
keyboard pagination focuses the results heading. English/Persian labels and the
existing theme/focus styles are shared. Resource links carry an exact source and
resource reference to a new question view; they neither replay an expired answer
nor start a run. Historic report replay is not supplied by metadata retention.

HTTP/client source `647c9ae` in run `42456685204` passes focused Go, actual SQL,
typed client and web gates, then fails source formatting. The hosted correction
also supplies canonical contract-derived types. It is applied before the desktop
increment; that newer combined source needs fresh full verification.

Desktop source `20be636` in run `42457050589` fails type checking because its
new tests use DOM matchers that this repository does not install. The correction
uses native element properties and ordinary assertions, preserving every behavior
check. That failure also blocks container/browser builds; it is one source defect,
not a container acceptance receipt. The hosted formatting patch is applied.
Review also closes the zero-length time-window gap in domain and client validation,
with matching negative tests; canonical windows require `to` strictly after `from`.

The real processor/application browser journey also opens this production history
page after each actual run in both languages and roles. It requires a successful
`no-store` metadata read, one to ten visible rows, no prior question/answer text and
no extra admission, then returns to the original resource. This new integration
check must pass on its owning source; it is not yet an acceptance receipt.

Source `a09a944` in run `42457323366` passes 27 focused transport tests and 163
focused desktop checks, including all five new history-page cases. Full source
verification stops at formatting in three files. Two corrections are retrieved
from the hosted artifact; the public barrel export is aligned by hand and added
to the hosted correction list. New source still needs a complete passing gate.

Preview source `d9251da` in run `42457637914` passes the same focused checks,
formatting, manifests, locales and contract derivation. Lint rejects a runtime
guard expressed against an already narrow input type and a redundant response
cast. The input validator now accepts `unknown` at its validation boundary, with
null/array/scalar negatives, and the return keeps the proven nullable cursor type.
No guard or lint rule is removed.

Operational review also finds the build-info metric fixed at `0.0.0`. The preview
uses the existing canonical application-version constant, preserves the fixed
metric shape and access policy, and adds a regression check. No user/source/run
identifier is added as a metric label.

Run `42455338720` at `8206322` reaches session revocation, then rejects the
fixture's missing reason. The fixture now sets `session.revoked` with its timestamp;
the real session constraint remains intact. Container jobs in that run also used
the superseded zlib pin; the primary runtime correction is merged before retesting.

## Remaining implementation contract

1. Add an immutable PostgreSQL migration for corpus metadata, approved current
   revision, document bytes and a bounded full-text index using fixed `simple`
   configuration. No extension, embedding store or filesystem crawler.
2. Require live administrator, CSRF, a dedicated recent-auth purpose, exact
   candidate digest/current version, explicit confirmation and atomic audit for
   approval/replacement/revocation. Validation or a browser approval boolean must
   never create read authority. Operators review secrets, prose and intended roles.
3. Restrict authenticated searches to currently approved revisions and permitted
   roles. Use parameterized full-text terms, exact locale, deterministic ordering,
   bounded keyset pages and a short read-only snapshot. Record actual query plans.
4. Bind every document reference to corpus ID/version/digest, document ID/locale
   and restore generation. Recheck current approval on opening a reference.
   Revoked/removed/expired versions report unavailable; old text is not reconstructed.
5. Add owner-only run metadata history with safe scope, status, timing and retained
   reference metadata. Omit questions, replies, generated reports, free prose,
   session material and foreign-owner rows. A prior administrator role grants no
   other owner's conversation content. Metadata retention remains bounded.
6. Add separate desktop views for search, approved version and run metadata.
   Use natural English/Persian, keyboard access and plain text. Expired text and
   unavailable documents get explicit states. No generated recommendation executes.
7. Exercise actual SQL import/search/replacement/revocation/restore, audience changes,
   cross-owner history, negative HTTP/auth/CSRF/recent-auth cases and bilingual
   browser flows on Actions. Check index/backup deletion semantics, source scans,
   race checks, package integrity and exact-tag publication before acceptance.

## Privacy and backup lifecycle

Only operator-reviewed document content may become durable. User conversations
are never a corpus source and are never learned automatically. Default remote
policy continues to exclude runbook prose. Replacing/revoking a corpus must remove
searchable old document/index content according to its documented retention policy.
Historical backups can retain old bytes until backup expiry; deletion must not be
promised as retroactive erasure. Restore fences approval and reference identities
before serving content. No new external store or outbound fetch is introduced.

## فارسی

این بخش فقط آماده‌سازی است؛ اعتبارسنجی، مجوز نمی‌سازد. هر نسخه به متن دقیق،
زبان، نام سند و نقش‌های مجاز بسته می‌شود. تأیید، تغییر و لغو به مدیر جاری،
احراز تازه، تأیید صریح و ثبت اتمی نیاز دارد. جست‌وجو فقط روی نسخهٔ تأییدشده
و با سقف روشن باشد. متن، دستور اجرا نیست. تاریخچهٔ حساب فقط فرادادهٔ همان
مالک را نشان دهد؛ پرسش و پاسخ منقضی‌شده بازسازی نشود. تست، ساخت و اسکن
فقط روی GitHub Actions. مسیر خواندن و صفحهٔ جدا آمادهٔ تست است؛ جست‌وجو،
تأیید سند و پذیرش کل گام هنوز باز است.
