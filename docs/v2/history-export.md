# Smart Assistant — selected private metadata export

Candidate **2.0.0-alpha.7.1**, schema **20**. Authored; its own hosted tests and
artifact acceptance are pending. No migration, retention, content replay,
inference request, provider call or new HTTP resource is added.

## Behavior and authority

On `/assistant/history`, select one to ten records on the displayed page, then
choose Download selected metadata. The display is only a selection aid. The
action performs a fresh existing owner-authorized history read with the exact
state, page size and cursor before constructing any downloadable bytes. Current
session, owner, restore epoch, seven-day retention and ten-minute cursor binding
remain the existing server boundary.

Every selected ID must still exist on that fresh page. If a run changed state,
expired, moved off a changing first page, or current access/restore/cursor changed,
the action fails without partial export. Refresh and select again. No silent
substitution, background pagination or automatic resubmission is allowed.

A closed JSON projection includes only selected run IDs, source/resource
references, goal/state, UTC record/window times, call counts and uncertain usage.
It excludes questions, responses, reports, evidence bodies, credentials, owner
IDs and paging cursors. Adding a transport field does not add it to the export.
The serialized cap is 32 KiB.

## File lifecycle

The explicit download is `stackplore-run-history.json`. Schema version1 has
`schema_version`, `exported_at` and `entries`. Export time uses the browser clock;
record timestamps come from the authorized response. This unsigned convenience
file is not a report, audit receipt or signed artifact.

Download follows a fresh read and final owner/cancellation check. Temporary
object URLs are revoked after one second or on unmount. Page, filter, locale,
owner and refresh changes unmount selection and cancel pending work; pagehide
clears the parent synchronously. No private bytes enter local/session storage or
application caches. The application cannot recall a file handed to the browser.
The user owns file custody/deletion. The notice says download started; it does
not claim the OS saved a file.

## Hosted verification

Unit cases cover fresh reads, closed projection, selected-only output, duplicate,
oversized/invalid selections, missing rows, owner change and late cancellation.
The real PostgreSQL history boundary stays unchanged. Real processor/browser
journeys download one selected record in both languages/roles, inspect actual
bytes and exact row keys, and assert no new admission or question content.
No local test, build or service is authorized.

## فارسی

در «تاریخچهٔ اجراهای من»، تا ده ردیف همین صفحه را انتخاب کنید و «دریافت
فرادادهٔ انتخاب‌شده» را بزنید. برنامه همان صفحه را با دسترسی جاری دوباره
می‌خواند. اگر حتی یک انتخاب جابه‌جا، منقضی یا نامجاز شده باشد، فایل ناقص ساخته
نمی‌شود؛ صفحه را تازه کنید و دوباره انتخاب کنید.

فایل فقط شناسه‌ها، زمان، حالت و شمار فراخوان‌ها را دارد؛ پرسش، پاسخ، گزارش، متن
شاهد یا مشخصات نشست در آن نیست. زمان صدور از ساعت مرورگر است؛ این فایل امضای
سرور یا اعتبار گزارش ندارد. نگهداری و حذف فایل دریافتی با کاربر است.


First candidate source `7f2f648` in run [42475542996](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42475542996) passed focused client/history/export checks and six install/provider/tool jobs; the full gate stopped at formatting. The hosted patch was applied. Additional interface tests exercise actual download initiation, temporary URL cleanup and a late response after pagehide; they still require their own fresh hosted receipt.

The same alpha7.1 candidate synchronously clears private document prose inside
pagehide, matching the existing history boundary. A regression inspects the DOM
in a later listener during the same event, rather than only after a batched render.
The publisher follow-up also avoids a redundant target parameter for verified
existing tags and rechecks the tag/gates before finalization. Both changes need
the candidate's fresh hosted gate; they are not part of immutable alpha7 source.

Candidate `bae47fa`, hosted run
[42478253971](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42478253971),
passed all 45 publisher unit cases and the focused client/interface checks.
The full gate stopped at formatting in three files; its hosted patch was applied.
The Kubernetes download failed after connection loss (curl56). The processor
receipt stopped at archive download, before any protocol/application case; both
cleanup flags are true. These are failed gates, not runtime passes. Transfers
now retry connection errors within fixed retry/time limits; all pinned hashes,
sizes, HTTPS restrictions and the 40-second application budget stay unchanged.

Follow-up `4d6776f`, run
[42478952710](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42478952710),
passed formatting and all focused cases but stopped at two unnecessary nullable
checks in the new pagehide test. Those checks were removed without weakening the
same-event privacy assertion. The Kubernetes asset still returned timeout/504;
the Helm install passed using the same pinned binary. Remaining jobs from this
superseded run are not accepted as the follow-up source's verification.

Source `e1c8721` in run
[42479540465](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42479540465)
passes the six install/provider/tool jobs, including Kubernetes. Focused cases
pass; the full gate stops at the one-line pagehide regression's formatting.
The hosted correction is applied; the privacy assertion remains unchanged.

Alpha7.1 source `6441c15eeab0a8c0ec98df96cd512021f95528a0` subsequently passes
all eight [source jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42479936460)
and its [signed package/security trial](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42479950922).
Its annotated tag is immutable. [Exact-source/tag/publication ledger](../evidence/2.0.0-alpha.7.1/README.md).
This accepts the bounded export/privacy implementation, not full stage7 or stable2.
