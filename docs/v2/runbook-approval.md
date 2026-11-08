# Smart Assistant — reviewed document approval and search

Development **2.0.0-alpha.7**, schema **20**. The repository/domain increment is
authored; no successful hosted receipt or HTTP/UI delivery is claimed yet.
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
The HTTP contract/client, management/search/document pages, independent human
quality, pilot and final stable acceptance remain open.

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
GitHub Actions؛ اتصال HTTP، رابط و پذیرش کل گام هنوز باز است.
