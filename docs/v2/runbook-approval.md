# Smart Assistant — reviewed document approval and search

Development **2.0.0-alpha.7**, schema **20**. Repository, use case and private HTTP routes are authored; no green full gate or browser delivery is claimed yet.
Migration20 is appended; earlier migration bytes remain immutable. Starting this
source applies schema20 even when the optional processor is disabled. Use only
isolated development data until its own upgrade/recovery and release gates pass.

## Authority and bounded material

At most 16 corpus identities (including revoked tombstones), each at most 32
English/Persian documents, 16 KiB per body and 512 KiB aggregate material. Reuse
an existing identity with its next revision when the identity bound is reached.
Versions are positive JavaScript-safe integers; version/audience/exact bytes are
bound into the canonical corpus digest. Candidate validation grants no approval.
Unapproved candidate bodies are not stored by the preview path.

A preview holds one metadata-only confirmation per session for two minutes. It
binds actor/session, corpus, expected previous revision, candidate digest, action
and restore generation. A new preview replaces that session's prior preview.
Approval/revocation then requires live administrator, explicit confirmation,
exact material, the one-use preview and `runbook.manage` recent authentication.
The preview, recent grant, corpus/documents, catalog revision and safe audit all
change in one transaction. A failure rolls all of them back. Audit stores fixed
action/reason and identity metadata, never title/body/query. Authentication,
session lifetime and current role are checked again before releasing results.

## Search and document references

PostgreSQL uses a generated `tsvector`, a GIN index and fixed `pg_catalog.simple`
configuration. `plainto_tsquery` receives a bound text parameter; query operators
and hostile prose remain terms/data. No stemming, embeddings, crawler, network
fetch, shell, automatic learning or external prose export is added.

Only approved current-generation corpora matching the live reader's role are
searched. Language is exact (`en` or `fa`); Arabic/Persian letter variants are not
silently normalized. Match ordering is deterministic corpus/document identity,
not an unmeasured relevance claim. Ten results plus one SQL sentinel are allowed.
Keyset pages fence catalog/restore revisions. A query is at most 256 UTF-8 bytes;
snippets are the first 512 UTF-8 bytes, preserving full code points. Search reads
at most eleven bounded 16-KiB bodies before projecting snippets. It does not
promise that the matched term appears in that prefix. Index/query plans and
performance claims require separate actual PostgreSQL measurements.

References bind corpus ID, document ID, locale, exact corpus version/digest and
restore generation. Every open rechecks session, current role, approval and exact
reference. Replacing audience/content, revocation or restore invalidates old
references and page state. Restore does not automatically approve old content;
the operator must explicitly review/import a newer revision. Approval has no
time-based expiry policy; it lasts until replacement, revocation or restore.

## Removal and backups

Replacement/revocation deletes old document rows, making old bodies and derived
index records unavailable to future authorized searches. PostgreSQL MVCC/dead
tuples, database files and older backups can still retain bytes until their own
vacuum/backup lifecycle; this is not a physical secure-erasure claim. Already
delivered bytes cannot be recalled. Corpus identity/version tombstones remain to
prevent stale-reference resurrection. Expired confirmations are pruned in bounded
batches and cascade away with removed sessions.

## Hosted proof and open integration

The authored real-SQL tests exercise unapproved/audience/locale filtering, exact
reviewed bytes, keysets, material substitution, revision replay, replacement,
revocation, restore fencing, SQL-injection-shaped search text, revoked sessions
and full rollback on audit failure. No test or formatter runs on the workstation.
The HTTP contract/client and management/search/document pages are authored and require fresh hosted browser acceptance. Independent human quality, pilot and final stable acceptance remain open.

First source `6ce64ad` in run `42466426878` fails a reader fixture that attempts
to bootstrap already initialized installation metadata. The correction inserts
only the additional reader/session. A separate verified kind download receives
HTTP504 on all three original attempts; the bounded retry window is expanded,
with HTTPS-only redirects and unchanged version/hash checks. Neither failure is
hidden or reused as acceptance. The hosted Go formatting correction for the test
is applied; newer repository/use-case bytes need their own complete gate.

The authored use case now owns two-minute confirmation tokens, exact repository
expiry, bounded authenticated ten-minute search cursors and shutdown cancellation.
Cursor signatures bind query, locale, page size and actor/session/revision without
putting query text or owner/session identifiers in cursor payloads. New unit tests
cover changed-scope cursors before I/O, expiry, wrong recent-auth purpose,
confirmation, malformed returned documents and late reads after key-clearing
shutdown. These are authored tests until the owning hosted source passes.

## فارسی

این نامزد، طرح پایگاه ۲۰ را اضافه می‌کند؛ با شروع برنامه حتی در حالت خاموش
اعمال می‌شود. روی دادهٔ مشتری آزمایش نشود. پیش‌نمایش فقط اثرانگشت و فراداده
را دو دقیقه نگه می‌دارد. متن تا تأیید نهایی ذخیره نشود. مدیر جاری، احراز تازه،
نسخه و متن دقیق، تأیید یک‌بارمصرف و ثبت اتمی لازم است. لغو یا جایگزینی، متن
قدیمی را از خواندن و جست‌وجوی مجاز بیرون می‌برد؛ پاک‌شدن فیزیکی یا حذف نسخه‌های
پشتیبان وعده داده نمی‌شود. بازیابی، تأیید قبلی را زنده نمی‌کند. جست‌وجو فقط
نسخهٔ تأییدشده و نقش مجاز را ببیند. متن سند دستور اجرا نیست. تست‌ها فقط روی
GitHub Actions؛ اتصال HTTP و رابط نوشته شده‌اند؛ نتیجهٔ دور و پذیرش کل گام هنوز باز است.

## Private HTTP boundary (authored, hosted verification pending)

The normal installation composes the document service independently of the optional
processor. Guest deployments omit it. Five private routes are declared in OpenAPI:
`GET /assistant/runbooks` for administrator metadata, and body-only POST routes
`/preview`, `/commit`, `/search`, `/document` below that path. All POST requests
require same-origin session and CSRF proof; commit also requires the dedicated
recent grant. There are no query parameters or browser-supplied owner identifiers.
The catalog exposes the effective request cap: min(deployment cap, 1 MiB), normally
256 KiB. The complete commit envelope, not just the corpus, must fit that cap.
Approval sends an empty digest and exact prose; the server computes its canonical
digest and binds it into the preview. Commit resubmits that same material. Revocation
uses the catalog's digest, current version and an empty audience/document stub.
Search returns ten or fewer plain-text snippets and an opaque cursor; opening a
result rechecks the exact revision and current rights. No HTML or Markdown is run.

Run `42467186114` at `bbba0b1` passed focused domain/use-case/transport and real
PostgreSQL checks. Its full gate stopped at Go formatting; the hosted correction
was applied. Its cluster jobs failed downloading the pinned tool with HTTP 504;
its processor receipt failed in the archive phase, before any runtime case. Both
release assets now have a second official transport with the same pinned digest.
These failed receipts remain failures; new source needs its own fresh full gate.
## Desktop document page (authored)

Open `/assistant/runbooks` from the assistant workspace. The Browse documents
section searches in the current interface language, replaces result pages, and
opens exact currently authorized references as plain text. Search terms and
references stay in request bodies, never navigation parameters or local storage.
The separate Review and approve section appears only for administrators. Load
current metadata, paste a bounded corpus JSON, review every exact document and
audience, acknowledge the change, and enter the current password. Editing the
candidate discards the preview. Leaving the page, changing account or language,
expiry, and cancellation clear private text and confirmation state. A network
failure during commit reports an unknown outcome and requires inspecting the
catalog; there is no automatic resubmission. Browser imports are additionally
bounded by the shared 256-KiB client cap and an envelope safety reserve.

Hosted unit and full-browser journeys are authored for English/Persian approval,
revocation, reopened version2, viewer reads, inert script-like text, query/body
privacy, changed owners, and late responses after pagehide. A passing unit test
alone does not establish the browser, PostgreSQL or publication gates. The API
client's generated shapes were obtained from run `42468005066`; no generator,
formatter, test, build or service ran on the user's host. That source's focused
transport and PostgreSQL jobs passed; full formatting corrections were applied.

## Correction trail and limits

Source `9ae27a3` passed focused Go and real PostgreSQL checks, but failed hosted
TypeScript checking: the second cancellation check was incorrectly narrowed across
an awaited request. The guard now performs a fresh read on each call. This also
blocked its image/install jobs; no install acceptance is claimed for that source.
The formatting patch came from run `42468642643` and was applied locally as a
file edit only. Follow-up checks cover expired/replaced confirmations without
spending a grant, the full 16-identity bound, and `EXPLAIN (ANALYZE, BUFFERS)` on
real PostgreSQL. A small synthetic plan is not a production capacity claim.
Metadata generation/version bounds and committed audience, counts and byte size
are checked before a response is released. The user has no independent evaluator
or pilot environment yet; later quality and stable acceptance still need those.

Source `24f1770` passed the focused client/page suites, real PostgreSQL expiry and
quota checks, Compose, Helm, container and provider checks. Its full code gate
stopped at formatting, so those passing focused results do not certify a full
release. The hosted correction from run `42468921345` was applied. Follow-up
review moves keyboard focus into the opened document/review and preserves the
unknown-outcome notice if the user leaves while confirmation is in flight.

## Import example and recovery

For a new identity, load the catalog and paste this candidate into the review page:

```json
{
  "id": "operations",
  "version": 1,
  "audience": "administrators",
  "documents": [
    {
      "id": "recovery",
      "locale": "en",
      "title": "Check service health",
      "body": "Inspect current health and recent events before drawing a conclusion."
    },
    {
      "id": "recovery",
      "locale": "fa",
      "title": "بررسی سلامت سرویس",
      "body": "پیش از نتیجه‌گیری، سلامت کنونی و رخدادهای اخیر را بررسی کنید."
    }
  ]
}
```

Use `all_users` only after reviewing whether every signed-in account may read the
prose. A later replacement must use the current version plus one, even following
revocation or database restore. To roll back content, take a trusted operator-held
copy, review it again, and approve it at the next version; never decrement a
revision, edit migration history or revive a tombstone. If recovering from backup,
inspect it in isolation and follow the normal installation restore procedure;
restored approvals remain fenced until a new explicit review. There is no public
API that recovers a replaced/revoked body from history or backups.

راهنمای کوتاه: از بخش دستیار، «سندهای تأییدشده» را باز کنید. برای جست‌وجو،
زبان صفحه باید با زبان سند یکی باشد. مدیر در «بازبینی و تأیید» ابتدا فهرست
کنونی را بگیرد، مجموعه را وارد کند، متن دقیق و گروه مجاز را بخواند و با
گذرواژه تأیید کند. تغییر متن، پیش‌نمایش را باطل می‌کند. پس از خطای ارتباط،
قبل از تلاش دوباره نسخهٔ فهرست را بررسی کنید. بازگردانی متن قدیمی هم یک
بازبینی تازه و شمارهٔ نسخهٔ بعدی می‌خواهد.

Signed package trial `42469270826` at `8f12dcd` stopped before scans: the pinned
scanner download returned HTTP504. The corrected downloader tries the same
immutable official API asset, enforces HTTPS, exact archive size and the unchanged
SHA256, then extracts the named executable. It does not skip a tool or weaken
scan severity. A new exact-source package/security trial is still required.