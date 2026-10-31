# Smart Assistant — status-only byte decoder

Development version: **2.0.0-alpha.4**, **2026-10-28**. This is a pure client
decoder. It does not open a connection or deliver the public status endpoint.
The separate [explicit connection reader](status-connection.md) now owns fetch,
timeouts, cancellation and cleanup. The [server subscription](status-server.md)
has separate authority and publication checks.

`createAssistantStatusDecoder(runId, afterCursor)` owns one run's decoding state.
The initial cursor defaults to `0`; accepted cursors are canonical decimal zero
through `9223372036854775807`. Invalid caller identities/cursors fail with fixed
`invalid_request`. Incoming faults fail with fixed `unexpected_response` and
permanently close the decoder; private source bytes are never attached to errors.

- Feed bounded `Uint8Array` chunks of at most **64 KiB** to `push`. The owning
  reader must split larger network chunks before passing them. A raw frame is
  at most **16 KiB**, including ignored comments and line separators. UTF-8 is
  strict; splitting a multibyte character across chunks is supported.
- LF, CRLF and CR line endings work across chunk boundaries. A BOM is allowed
  only at the stream start. Bounded comment heartbeats yield no messages.
- Data frames contain one `event`, one single-line JSON `data`, and a status
  `id`. Reset may omit `id`; an optional reset ID must be canonical and never
  advances replay. Reject unknown fields (including `retry`), repeated fields,
  duplicate JSON keys including escaped aliases, multiline data, unknown event
  kinds, different run IDs and any question/answer/citation fields.
- A status frame yields `{type: 'status', id, data}` with only the closed
  `AssistantStatusEvent` shape. Its ID must equal its sequence, with sequence and
  version both integers **1–64**. All eight durable states remain distinct.
  Versions cannot move backwards during a decoder's lifetime. From cursor zero,
  the first authorized snapshot may jump to the current sequence; subsequent
  statuses must be contiguous. A nonzero cursor always requires contiguous replay.
- A reset yields `{type: 'reset', data}` with one of the four closed reasons.
  Later status/data frames are rejected. The owning flow must close its reader
  and perform a fresh private REST read; reset never authorizes a new run or a
  resend. A future cursor therefore expects reset, not an invented status.
- `push` returns messages only after every frame in that chunk passes. A bad
  trailing frame throws without returning an earlier valid prefix from that
  same call. Already delivered metadata from earlier chunks is not retracted.
- On clean EOF, consume the messages returned by `finish()` too; a terminal CR
  may complete the final event. An unfinished frame fails instead of silently
  becoming a completed status. Use idempotent `close()` on cancellation or owner
  changes; it erases pending bytes/fields and rejects all late pushes.

The decoder never renders text, chooses tools, opens URLs, stores data, connects,
reconnects, polls, refetches or resends. Server authority and queue/write/lifetime
bounds remain required. The owning application must bound its connection,
subscription count and reader lifetime, abort it on owner changes, and use the
[closed REST reply boundary](conversation-replies.md) before displaying a complete
answer. This increment does not provide that owning network/UI flow.

## فارسی

این بخش فقط خواندن قاب وضعیت است؛ اتصال یا مسیر عمومی نسازد. حد قاب ۱۶ KiB
و حد هر تکه ۶۴ KiB است. مرز تکه، حرف چندبایتی و پایان خط بررسی شوند. فقط
شناسه، شماره، نسخه، حالت و زمان همان کار برگردد؛ متن سؤال و پاسخ جایی در
خروجی ندارد. فیلد تکراری یا ناشناخته، ارجاع به کار دیگر و ترتیب غلط رد شود.

reset نیاز به خواندن تازهٔ وضعیت خصوصی دارد؛ اجازهٔ اجرای دوباره نیست.
صفحه باید خواننده را ببندد، مهلت و شمار اتصال را محدود کند و با خروج یا عوض
شدن کاربر `close()` را بزند. در پایان سالم، خروجی `finish()` نیز خوانده شود.
قاب نیمه‌کاره خطا بدهد؛ متن آن در خطا نیاید. صفحه و مسیر سرور، بخش‌های جدا با شاهد مستقل‌اند.
