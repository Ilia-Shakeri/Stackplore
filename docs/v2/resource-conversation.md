# Smart Assistant — selected-resource conversation

Development **2.0.0-alpha.4**, schema **18**, 2026-10-31. This source adds the
owning bilingual view to a resource detail page. Full verification, real browser
checks and exact-commit hosted evidence are separate gates; this document does
not mark stage 4 or version 2 complete. See [development evidence](../evidence/2.0.0-alpha.4/README.md).

## Use

1. Sign in, open **Resources**, and select one resource. Administrators and
   viewers can open the view; guest mode has no conversation controls.
2. Open **Smart Assistant**, choose the evidence window, and enter a question.
   Opening or typing performs no conversation request. **Send question** starts
   one explicit submission, with at most one bounded status subscription for an
   active run and a fresh result read. The feature requires the operator's enabled local profile;
   an unavailable profile or evidence is shown truthfully.
3. Read the separate facts, possible explanations, gaps and suggested checks.
   The answer is plain text. **Check resource evidence** resolves current
   authority again, checks the exact resource/range, then focuses the selected
   resource heading. It does not follow a model-provided URL or execute a check.
4. If live status stops, explicitly read the current result or status again.
   Neither action resends the question. If submission is uncertain, **Retry the
   same request** preserves its original body and idempotency key. Repeated
   clicks cannot replace that question or create an automatic retry loop.
5. **Stop and clear text** removes local text before waiting for a protected
   cancellation acknowledgement. A completed run is reported as already ended;
   it is not relabelled cancelled. Unknown cancellation remains unknown.
   Closing the view clears local text and subscription state; it does not prove
   durable server cancellation. Server work remains subject to its own bounds.

## Lifetime and privacy

A question is at most **8 KiB of valid UTF-8**, without NUL or lone surrogate
characters. The view holds at most **eight submitted turns**, with a **30-minute**
deadline from the first private input, including an unsent draft. Editing or
retrying does not extend it. A new conversation requires explicit new input.
If server content has been lost, the next explicit question starts a fresh
server conversation; old text is never automatically restored or resent.

Scope is the selected source, resource, time window and language. A changed
scope or opaque session owner creates a new empty view. Logout, owner/authority
changes, unmount, page exit and expiry drop private references and cancel owned
reads. The page-exit handler flushes the empty state before the page can be
frozen. Returning from browser history cannot revive the old draft. A resumed
or visible page also checks the absolute deadline when timers were suspended.
Memory reference release is not a claim of forensic erasure of browser memory.

Questions, answers, conversation IDs, retry keys and status cursors have no URL,
local/session storage, offline-cache, telemetry or persistence path in this
view. The public service worker bypasses every `/api/` request. The existing
metadata-only database and volatile service policy still apply. Do not enter
credentials or private keys in questions. Plain-text rendering, complete reply
validation and fresh server authority remain independent defenses.

English/Persian labels and status messages share one catalog. Technical
identifiers/ranges are LTR-isolated; answer and question paragraphs use automatic
text direction. Standard controls support keyboard use, visible focus and
mobile wrapping. Existing theme tokens supply both themes; no motion is added.
Automated accessibility checks do not establish complete assistive-technology
or product acceptance.

## Implementation and evidence

[Browser ownership](browser-ownership.md) supplies the private typed port.
`AssistantConversation` owns one volatile scope, frozen retries, bounded turns,
expiry, status recovery, current citations and late-result fences. Constructor
and mounting perform no content I/O. Cleanup increments its epoch before
discarding state; effect remount starts empty. A retained status version also
fences an older content read. Terminal metadata cannot silently change state.
Citation completion is also fenced to its view lifetime, exact snapshot and
latest explicit check. An old failed check cannot add a warning to a new scope
or move focus after the current view has changed. Warnings belong only to the
snapshot that produced them.

The router composes the port from its own session store. The resource panel is
absent outside authenticated context, and is collapsed initially. There is no
remote endpoint/profile picker, free-form remote interpolation, investigation
loop, durable conversation history or autonomous infrastructure action.

Unit fixtures check scope/owner/expiry, ambiguous admission, cancellation,
reset/EOF, malformed text, stale replies, citation binding, localization and
plain-text display. The browser journey uses the production application/client
and actual disposable PostgreSQL for normal authentication and exploration,
but **synthetic intercepted private HTTP replies** for conversation. It checks
both roles/languages, mobile keyboard operation, both themes, accessibility,
current citation focus, history re-entry and lack of conversation resend. It
starts no processor. Those fixtures cannot prove current engine operation,
grounding, human output quality, production workload, soak or stage acceptance.

## فارسی — راهنمای منبع انتخاب‌شده

پس از ورود، در «منابع» یک منبع را باز کنید. مدیر و مشاهده‌گر می‌توانند دستیار
را باز کنند؛ مهمان این کنترل‌ها را ندارد. بازکردن و تایپ، پرسشی نمی‌فرستد.
بازه را انتخاب و «ارسال پرسش» را بزنید. فعال‌بودن نمایهٔ محلی و دسترسی به
شواهد لازم است؛ خطا یا نبود داده به‌روشنی نمایش داده می‌شود.

واقعیت، احتمال، کمبود شواهد و بررسی پیشنهادی جدا نمایش داده می‌شوند. متن
اجرا نمی‌شود. «بررسی شواهد منبع» مجوز، شناسه و بازه را تازه می‌سنجد و عنوان
همان منبع را در کانون می‌گذارد؛ پیوندی از متن پاسخ دنبال نمی‌شود.

پس از قطع وضعیت، نتیجه یا وضعیت را صریح دوباره بخوانید؛ پرسش تکرار نمی‌شود.
اگر نتیجهٔ ارسال نامعلوم بود، فقط «تکرار همان درخواست» همان بدنه و کلید را
نگه می‌دارد. «توقف و پاک‌کردن متن» پیش از رسید لغو، متن محلی را پاک می‌کند.
پایان قبلی اجرا، لغو تازه نام نگیرد؛ لغو نامعلوم هم نامعلوم بماند. بستن نما
به‌تنهایی تأیید لغو سرور نیست.

حد هر پرسش ۸ کیبی‌بایت، سقف ارسال هشت نوبت و عمر نما سی دقیقه از نخستین
ورود متن است. ویرایش و تکرار زمان را تمدید نکنند. با تغییر منبع، بازه، زبان
یا نشست، خروج، بستن صفحه و انقضا، نما پاک می‌شود. بازگشت تاریخچه متن را
زنده نمی‌کند. گم‌شدن متن در سرور به ورود صریح پرسش تازه نیاز دارد.

متن و کلید تکرار در نشانی، ذخیرهٔ مرورگر یا پوستهٔ آفلاین ذخیره نمی‌شوند.
شناسه‌های فنی و بازه جهت مستقل دارند؛ تم، فوکوس و کنترل‌ها از سامانهٔ مشترک
پروژه‌اند. تست مرورگر از پاسخ‌های خصوصی ساختگی استفاده می‌کند؛ شاهد موتور
واقعی، کیفیت انسانی، ظرفیت یا پایان مرحله نیست. نسخهٔ دو هنوز کامل نشده است.
