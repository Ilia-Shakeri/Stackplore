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
