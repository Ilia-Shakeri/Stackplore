# Smart Assistant — reviewed runbooks and metadata history

Status: **stage-7 preparation only**. The pure corpus model, owner-metadata read
port and boundary tests are authored on `v2/runbooks-prep`. They expose no HTTP
route, create no database table and do not complete stage 7. Stage-6 quality review and
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
preparation has no search engine or approval state. It performs no I/O.

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
credential is selected. This port is not wired to the HTTP or desktop boundary yet.

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
فقط روی GitHub Actions. مسیر SQL، رابط و پذیرش کل گام هنوز آماده نیستند.
