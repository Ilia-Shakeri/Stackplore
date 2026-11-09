# Smart Assistant — reviewed document approval and search

Current candidate schema **22**, version **2.0.0-alpha.7.3**. Selected-document
guidance requires fresh hosted proof. Published alpha7.2 provenance passes source,
exact-tag and publication gates: [receipts](../evidence/2.0.0-alpha.7.2/README.md).
The bounded alpha7 source `94425f8` passes all eight source jobs
and a signed package/security trial. Exact-tag attempt2 passes; its first
40-second timeout remains recorded. [Source, tag and publication evidence](../evidence/2.0.0-alpha.7/README.md)
is authoritative for release state. This guide alone grants no acceptance.
Alpha7.1 source `6441c15` passes all eight source jobs and signed trial
`42479950922`; its immutable tag has separate verification/publication gates.
It adds [selected metadata export](history-export.md) and synchronous page-exit
clearing. Provenance is a later increment, outside that immutable source.

## Material and review

At most 16 corpus identities, including revoked tombstones, each with 32
English/Persian documents. IDs are lowercase slugs of at most 64 ASCII characters;
versions are positive JavaScript-safe integers. Titles are at most 256 UTF-8
bytes, bodies 16 KiB, and aggregate identity/audience/provenance/document text 512 KiB.
Bodies allow LF/tab and valid visible Unicode; joiners remain valid Persian text.
The domain-separated, length-framed digest binds identity, version, audience and
exact document bytes independent of input order. The v2 digest binds the source
review; new v3 digests also bind separate processing permission. Existing stored
digests are preserved. Validation grants no approval.

A preview stores only confirmation metadata for two minutes, one per session.
It binds actor/session, action, expected previous version, digest and restore
generation. A newer preview replaces the prior one. Draft prose is not persisted.
Approval/revocation needs current administrator, explicit acknowledgement, exact
material, the preview and a dedicated one-use `runbook.manage` recent-auth grant.
Confirmation, grant, content, catalog version and safe audit commit atomically;
any failure rolls them all back. Audit contains fixed identity/action/reason
metadata, never document title/body/query. Prose approval is an operator review,
not a semantic accuracy or legal-rights certificate. The current document-to-answer
candidate follows [ADR-0023](../adr/0023-selected-document-guidance.md); its own
hosted acceptance is pending.

Every new approval needs collection-level `provenance`: `kind` is
`operator_authored`, `licensed_copy` or `public_domain`; `source` is visible inert
attribution up to 512 UTF-8 bytes; `license` records the usage basis up to 128
UTF-8 bytes; `use_reviewed` must be true; `assistance_reviewed` must explicitly be
true or false. Only true permits local assisted processing after user consent.
These are operator assertions. Use a
separate collection when source/usage bases differ. No source address is fetched.
The review checkbox acknowledges exact content, audience and recorded usage rights.

Migration21 preserves schema20 revisions with nullable provenance and unchanged
digests. Authorized readers may still open their inert text, with an explicit
missing-review notice. No new approval may omit the source review. Reapprove a
new version before assisted use. Migration22 defaults separate processing
permission to false for every existing revision; it never infers expanded rights.
Read responses contain separate `document` and nullable `provenance` objects;
catalog rows also expose nullable provenance. Imports keep one record per corpus.
Revocation retains bounded attribution as collection metadata while removing
live document rows; it does not claim retroactive erasure of metadata or backups.

First provenance source `214ad4b`, hosted run
[42482579739](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42482579739),
passes focused Go and real PostgreSQL checks, including the new migration20 to21
preservation and source-substitution regressions. The early client check stops
because committed derived types still describe the earlier contract. The hosted
generator/format artifact is applied; it is not generated on the workstation.
Container builds depending on those stale types fail in the same attempt. Fresh
full verification of the combined source is required; no failure is relabelled.

## Private HTTP and desktop flow

All paths below are relative to `/api/v1`:

| Method and path | Scope |
| --- | --- |
| `GET /assistant/runbooks` | Administrator catalog metadata and effective request cap |
| `POST /assistant/runbooks/preview` | Exact approval or revocation preview |
| `POST /assistant/runbooks/commit` | Explicit confirmation with dedicated recent grant |
| `POST /assistant/runbooks/search` | Current-account language/audience search |
| `POST /assistant/runbooks/document` | Exact currently authorized document read |

All POSTs require same-origin session and CSRF proof. There are no query
parameters or caller-selected owner fields. Commit also needs recent auth.
The effective request cap is min(deployment cap, 1 MiB), normally 256 KiB; the
complete envelope must fit. The browser additionally keeps its shared 256-KiB
client cap and an envelope reserve. Other deployment HTTP limits still apply.
Guest deployments omit the service; normal installations compose it independently
of the optional processor. Approval, search and manual document endpoints start
no source/provider or inference calls. The separate selected-question path below
requires explicit consent and its own admitted run.

Open `/assistant/runbooks` from the assistant workspace. Browse documents searches
in the interface language and replaces result pages. Opening a result performs
fresh authorization and displays plain text, without HTML/Markdown execution,
link activation or host/URL fetching. Administrators use Review and approve,
load current metadata, paste a corpus, inspect all text and audience, acknowledge,
and enter their current password. Editing discards the preview; expiry, owner,
language and page changes clear private state and cancel work. In the alpha7.1
candidate, pagehide clears synchronously before its event finishes. Interrupted
confirmation reports an unknown outcome: inspect current catalog/version before
preparing another preview. Never automatically resend the old confirmation.

## Search and references

PostgreSQL uses a generated tsvector, GIN index, fixed `pg_catalog.simple` and
parameterized `plainto_tsquery`. Search terms and hostile prose remain data.
Only approved current-generation corpora matching live reader role are searched.
Language is exact `en`/`fa`: no Persian/Arabic letter normalization, stemming,
embeddings, relevance ranking, crawler or automatic learning is supplied.

Queries are at most 256 UTF-8 bytes, pages one to ten hits. The adapter reads at
most eleven bounded bodies, then returns the first 512 UTF-8 bytes as snippets,
preserving code points; a snippet need not include the matched term. Ordering and
keysets explicitly use `COLLATE "C"`, independent of database default collation.
Cursors bind actor/session/role/revision, query, locale, size, restore and catalog
version for ten minutes. Query text and unkeyed query hashes are not serialized.
Synthetic `EXPLAIN (ANALYZE, BUFFERS)` proof is not production capacity evidence.

References bind corpus ID/version/digest, document ID/language and restore
generation. Every open rechecks live session/role, audience, approval and exact
reference. Replacement, revocation and restore invalidate old references; no
fresh content silently replaces an old witness. A restored metadata approval
is labelled as requiring new review and cannot be read until reapproved at a
newer version. Approval has no time-based expiry; it lasts until those changes.

## Import, replacement and recovery

A new administrator-only collection can use this JSON:

```json
{
  "id": "operations",
  "version": 1,
  "audience": "administrators",
  "provenance": {
    "kind": "operator_authored",
    "source": "Operations team's original installation notes",
    "license": "Author permits this installation's document use",
    "use_reviewed": true,
    "assistance_reviewed": false
  },
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

Use `all_users` only if every signed-in account may read the prose. Approval
sends action `approve`, current expected_version and generation, an empty digest,
and that corpus; the server computes the exact digest returned by preview.
Commit resubmits the same material with its confirmation and acknowledgement.
Revocation uses action `revoke`, the catalog's version/digest, and a corpus stub
with the same ID/version, empty audience, empty document array and null provenance.

Replacement or recovery always uses current version plus one, including after
revocation/restore. Keep trusted operator-held copies. To roll back prose, review
an old copy and approve it at the next version; never decrement versions, edit
migration history or revive a tombstone. Inspect backups in isolation and use the
normal installation restore procedure. Restored content is not auto-approved.
There is no history endpoint that reconstructs replaced/revoked document bodies.

Replacement/revocation deletes old live document rows and derived index entries
from future authorized reads. MVCC/dead tuples, WAL, replicas and backups retain
their own lifecycle; no physical secure erasure or recall of delivered bytes is
claimed. Identity/version tombstones prevent stale-reference resurrection.
Expired confirmations are pruned in bounded batches and cascade with sessions.

## Smart Assistant — selected-question path and upgrade

Deploy matching web/server assets and apply forward-only migration22 after a
verified backup. The nullable `document_reference` citation field and explicit
boolean `assistance_reviewed` provenance field are required by the new client.
Existing approvals remain reference-only; to enable processing, review usage
rights and approve a new version with `assistance_reviewed: true`.

On a resource page choose a guide, search in the current language, open its exact
revision, review the 256-UTF-8-byte excerpt and explicitly consent. The next
question attaches only that reference. Log consent and guide selection cannot
be combined. The local profile reserves one additional read within the same
eight-read cap. Explain uses two reads; incident uses eight. A guide citation
opens a newly authorized exact witness. Guidance never enters observed facts
or deterministic report timeline rows. No question, excerpt or answer is added
to retained history metadata or exports. Closing the view or losing account,
language, resource or page ownership clears private state and cancels pending work.

## فارسی

از بخش دستیار، «سندهای تأییدشده» را باز کنید. زبان صفحه باید با زبان سند یکی
باشد. مدیر در «بازبینی و تأیید» فهرست کنونی را بگیرد، متن دقیق و گروه مجاز را
بخواند، سپس با گذرواژه تأیید کند. تغییر متن، پیش‌نمایش را باطل می‌کند. پس از
خطای ارتباط، پیش از تلاش دوباره نسخهٔ فهرست را بررسی کنید. تأیید قبلی را
خودکار تکرار نکنید. برگشت متن قدیمی هم بازبینی تازه و نسخهٔ بعدی می‌خواهد.

متن سند دستور اجرا نیست. خواندن دستی، پردازش را شروع نمی‌کند. در پیش‌نسخهٔ ۷٫۳،
فقط مدیر با بازبینیِ نسخهٔ تازه و assistance_reviewed برابر true مجوزِ جداگانهٔ
پردازش محلی می‌دهد. سندهای قبلی پس از مهاجرت ۲۲ این مجوز را ندارند. در صفحهٔ
منبع، سند هم‌زبان را جست‌وجو و باز کنید، چکیده را ببینید و با رضایت صریح به
پرسش وصل کنید. سقف چکیده ۲۵۶ بایت UTF-8 است. سند و رضایت لاگ هم‌زمان نباشند.
شاهدِ سند با دسترسیِ تازه باز شود؛ متنش واقعیتِ مشاهده‌شده یا ردیف گزارش نشود.
متن خصوصی به تاریخچه و خروجی فراداده افزوده نشود. وب و سرور با هم ارتقا یابند
و پیش از مهاجرت پشتیبانِ تأییدشده بگیرید. این مسیر هنوز تست دور کامل می‌خواهد.
بازیابی، تأیید قدیمی را زنده نمی‌کند. حذف از
خواندن مجاز به معنی پاک‌شدن فیزیکی نسخه‌های پشتیبان نیست. تست، ساخت و اسکن
فقط روی GitHub Actions است. نتیجه‌های ساختگی، ارزیابی مستقل یا پایلوت نیستند.
