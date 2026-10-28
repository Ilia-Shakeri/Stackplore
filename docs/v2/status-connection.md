# Smart Assistant — one explicit status connection

Development **2.0.0-alpha.4**, 2026-10-28. `readAssistantStatus` connects the
[bounded decoder](status-decoder.md) to one explicit same-origin GET. This is
the client connection component; the server subscription and owning view are
not yet delivered.

The caller supplies a canonical run ID, optional replay cursor and a short
synchronous metadata callback. The request has same-origin mode/credentials,
`no-store`, redirect rejection and no referrer. It contains no question or
mutation capability. The callback must return `undefined`; returned values,
promises and faults produce a fixed transport failure. Callers must keep the
callback brief; JavaScript cannot interrupt their synchronous code.

Status 200, event-stream media type, no-store response and a readable body are
required. Declared and actual data are bounded to 1 MiB; at most 65,536 read
chunks prevent empty-chunk loops from starving timers. Large transport chunks
are split before the 64 KiB decoder input limit. Frame/schema/UTF-8/run/replay
checks remain in the decoder. No partial chunk prefix is published when that
decoder chunk has an invalid suffix.

The total lifetime is at most 120 seconds and idle wait at most 15 seconds;
callers may lower both, with idle no greater than lifetime. Waiting for fetch
headers and body reads is independently cancelled even if a supplied transport
ignores abort. Each wait removes its cancellation listener; there is no retained
queue of completed waits. Late responses have their bodies cancelled. Reset,
EOF, owner abort, timeout and malformed data close/wipe the decoder, cancel the
reader and release its lock. Cleanup does not await an uncooperative stream's
cancel promise. Aborts never expose the caller's private reason.

Callbacks are fenced before and after each publication, including a same-chunk
owner change. A reset returns `reset`; clean EOF returns `ended`. Neither causes
automatic reconnect, content fetch or question resend. The future owner must
decide when to refresh metadata and must supply a fresh ownership signal after
logout/account/source changes. No text or cursor is written to persistent or
offline storage.

Tests cover request protection, replay, metadata rejection, malformed suffixes,
large chunks, final CR framing, byte/read-count bounds, stalled transports,
late cleanup, heartbeat versus total lifetime, callback faults and owner
revocation. These controlled transport tests do not prove the server endpoint,
real network behavior, browser ownership integration or whole-stage delivery.

## فارسی

خواندن وضعیت، یک درخواست صریح و محدود باشد. فقط فراداده بدهد؛ سؤال نفرستد،
متن ذخیره نکند و وصل‌شدن یا ارسال دوباره خودکار نداشته باشد. سقف کل ۱۲۰ ثانیه،
سکوت ۱۵ ثانیه و داده یک مگابایت است. لغو، تغییر مالک، قطع و پاسخ نادرست،
خواننده را ببندند؛ پاسخ دیررس هم دور ریخته شود. خطا دلیل خصوصی لغو را ندهد.
اتصال سمت سرور و صفحهٔ صاحب این جریان هنوز مانده‌اند.
