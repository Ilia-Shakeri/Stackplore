# Smart Assistant — browser ownership boundary

Development **2.0.0-alpha.4**, schema **18**, 2026-10-31. This boundary is a
prerequisite for the owning resource conversation. It does not mount a view,
start a request, open a stream or deliver the complete stage.

## Session identity

`AuthStore.getSessionOwner()` returns an opaque in-memory symbol, or null
without an authenticated session. It contains no user ID, cookie, proof or
question and cannot be used as server authority. Ordinary same-session activity
refreshes preserve the symbol. Proof, account, role, user revision or enabled
state changes replace it before subscribers run. Explicit login, setup and
password replacement also replace it. Logout, retry, expiry and unavailable
session states clear it. Returning to the same account never revives a cleared
symbol. Existing protected action runners also recheck this identity before
accepting late results; recent-auth grants must pass the check before dispatch.

## Typed private calls

`createAssistantApi` binds the existing closed request/reply client to the
session store. Construction performs no I/O. Each explicit admission, read,
cancel, citation or status call requires the initiating owner symbol. A stale
owner is rejected before dispatch, including cancellation: a replacement
session cannot cancel the old owner's run through this port.

REST calls have a **15-second** local lifetime. A status subscription has a
**120-second** outer lifetime in addition to its existing byte, idle and frame
bounds. Caller abort and observed owner changes cancel the owned request and
settle its local promise even if a transport ignores abort. Late resolution or
rejection remains observed, but cannot publish content. A final owner check
also catches missed subscription notification. All paths remove listeners and
timers and dispose the owned abort controller. These are local cancellation
bounds; they do not prove when an unseen server permission change is detected.

Mutation protection remains inside `runSessionAction`; the port never exposes
the proof in view state. Admission preserves the caller's explicit immutable
request and nonce; it generates neither replacement requests nor retry keys.
Status consumers must finish synchronously and return undefined. Owner checks
surround each callback; a changed owner or disposed subscription cannot accept
a late frame. Returned promises are observed and rejected as invalid consumers.

There is no cache, storage, URL serialization, background read, automatic
reconnection, question resend or raw-error display in this boundary. The [owning resource view](resource-conversation.md) must additionally erase its input, turns and scope state on
navigation, owner change, expiry, page exit and unmount; bound the conversation
to eight turns or 30 minutes; render inert text and resolve citations afresh.
This port alone does not implement those view obligations.

## Verification boundary

Focused tests use controlled request/status functions, real session-store
transitions and fake timers. They cover guest denial, identity rotation,
ordinary refresh, protected mutation, stale cancellation, ignored abort,
pre-dispatch cancellation, timeout, missed notification and late status
callbacks. Strict types, lint, measured branch coverage and a complete clean
snapshot gate remain separate required evidence. Protocol fixtures do not
prove a real processor, browser journey, human grounding or full acceptance.

## فارسی

نشانِ مالک فقط در حافظه است؛ رمز، کوکی و متن ندارد. تمدید عادی نشست نشان را
نگه دارد؛ تغییر حساب، نقش، نسخهٔ کاربر یا نشست آن را پیش از خبر به صفحه
عوض کند. خروج، انقضا و خطای نشست نشان را پاک کنند. ورود دوباره نشانِ کهنه
را زنده نکند. پاسخِ دیررسِ کار محافظت‌شده هم دوباره مالک را بسنجد.

هر درخواست صریح به همان مالک بسته است. لغو با نشستِ جایگزین انجام نشود.
حد محلی REST پانزده ثانیه و جریان ۱۲۰ ثانیه است. قطع و تغییر مالک درخواست
را ببندند؛ حتی اگر شبکه لغو را نپذیرد، پاسخ دیررس به صفحه نرسد. شنونده و
زمان‌سنج در هر راه پاک شوند. کلید تکرار و بدنه خودکار عوض یا ارسال نشوند.

این بخش هنوز صفحه نیست. پاک‌شدن متن با تغییر منبع و بازه، خروج و بستن صفحه،
سقف هشت نوبت یا سی دقیقه، متن بی‌اجرا و ارجاع تازه در گام نمای گفت‌وگو لازم
است. تست نمونه و امتیاز کد، شاهد موتور واقعی یا پایان نسخهٔ دو نیست.
