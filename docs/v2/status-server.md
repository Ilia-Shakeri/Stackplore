# Smart Assistant — private status subscription

Development **2.0.0-alpha.4**, schema **18**, 2026-10-28. The server registers
`GET /api/v1/assistant/runs/{run_id}/events` only when its trusted run dependency
also supplies `AssistantStatusReader`. Enabled application composition supplies
that coordinator. Default-off and guest behavior remain sealed. This component
does not deliver the bilingual owning view or whole-stage acceptance.

## Authority and replay

The route accepts one canonical run ID and an optional canonical decimal
`Last-Event-ID` from zero through `9223372036854775807`. It rejects queries,
bodies, duplicate headers, aliases, signs, whitespace and overflow before
metadata I/O. Authentication constructs the actor from consistent current
session metadata. Each page reauthenticates that same actor and rechecks SQL
owner/session/source/profile/policy/restore authority. An account/session change,
revocation or changed retained scope closes the subscription. Storage faults
remain distinct from denial; diagnostics never include source text.

The coordinator reads durable metadata directly, never a volatile answer,
evidence, provider or processor. Missing/zero cursor produces one current
version snapshot. Nonzero cursors read retained committed status rows, then
recheck current authority and publication bounds. Sequences must be contiguous,
versions/timestamps ordered, and the current tail must match current state.
Rows are detached. A replaced or regressed run cannot publish a replay prefix.

| Reset reason | Condition |
| --- | --- |
| `cursor_ahead` | Valid cursor exceeds current durable version |
| `replay_gap` | A required retained sequence is absent |
| `replay_expired` | Retained replay is unavailable and latest metadata is older than the 24-hour replay window |
| `queue_overflow` | The typed replay exceeds the maximum 64 status versions |

Reset data contains only run ID and the closed reason; it emits no status ID
and closes. A fresh REST read is required. It never resends a question. Metadata
can outlive volatile content; a snapshot alone does not claim an answer exists.

## Transport bounds

The independent subscription gate allows **16** active streams and **2** per
owner. Full 128-byte valid owner identities remain valid after their internal
namespace is added. Rejected subscribers perform no metadata read. Initial
authentication/page reads have three-second bounds; each later page shares a
fresh three-second bound. The subscription lifetime is at most 120 seconds,
polling every two seconds, with ten-second comment heartbeats.

There is no background producer or unbounded queue. A complete validated replay
batch is assembled before publication: at most 64 statuses, 16 KiB per frame and
64 KiB per batch. Writes/flushes require enforceable deadlines, bounded by five
seconds and the remaining subscription lifetime. On return, at most one extra
second is permitted for the HTTP server's final chunk terminator, with no new
metadata read or frame. An expired write deadline must not turn clean EOF into
a truncated response. Disconnect/lifetime cancellation stops pending reads and
releases the owner slot. No new background producer requires joining.

Responses are `text/event-stream`, `no-store`, `Vary: Cookie`, with proxy buffering
disabled. The only data frames are `assistant.status.v1` with five closed
metadata fields, or `assistant.reset.v1`. A terminal current tail ends cleanly.
Heartbeat, EOF and reset contain no question, answer, citation, native identity
or resource payload. The [explicit client reader](status-connection.md) owns
its separate fetch/byte/time/owner limits; it does not reconnect automatically.

## Evidence boundaries

Real TCP tests check framing, no-store headers, replay/reset shape, deadlines,
EOF, lifetime cancellation, quota, revocation/scope replacement and sanitized
errors. A shared [native publication fixture](../../contracts/fixtures/assistant-status.json)
is also decoded by the client. A real PostgreSQL/public-admission fixture checks
snapshot, all retained statuses, future cursor, an intentionally removed owned
sequence and SQL actor revocation. Provider/processor counters stay unchanged
during status requests. Its authentication and processor remain synthetic;
database authority, coordinator, transport and adapter code are real.

This is not a complete real-user browser journey, current engine proof,
human-quality, accepted branch measurement, slow-client load/capacity, soak or
release acceptance. Separate complete source and hosted receipts are required.

## فارسی

جریان فقط با وابستگی مورد اعتماد وصل شود؛ پیش‌فرض و حالت مهمان بسته بمانند.
فقط شناسهٔ اجرا و نشانگر عددی درست پذیرفته شوند. هر خواندن، نشست و مالک و
اختیار پایگاه را تازه بسنجد. متن سؤال، پاسخ، ارجاع و دادهٔ منبع وارد جریان
نشوند. نشانگر جلو، تاریخچهٔ منقضی، شکاف یا سرریز فقط reset محدود بدهند.

سقف ۱۶ اتصال و دو اتصال برای هر مالک است. خواندن سه ثانیه، عمر ۱۲۰ ثانیه،
قاب ۱۶ کیلوبایت و بسته ۶۴ کیلوبایت حد دارد. پایان شبکه فقط یک ثانیه فرصت
اضافی برای بستن درست بسته داشته باشد؛ خواندن تازه انجام نشود. لغو و قطع،
ظرفیت را پس بدهند. شناسهٔ مجاز بلند هم به خاطر پیشوند داخلی رد نشود.

تست شبکه و پایگاه واقعی پاس لازم دارد؛ هویت و پردازنده در تست ترکیبی
آزمایشی‌اند. صفحهٔ دوزبانه، موتور واقعی و پذیرش کل مرحله هنوز مانده‌اند.
