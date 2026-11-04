# Smart Analysis — selected incident history

Development candidate **2.0.0-alpha.6.2**, schema **19**. Execution and acceptance
receipts must identify their exact source; authored checks are not passing receipts.

The user starts one selected-resource investigation. Its fixed ordered reads are
inventory, health, metrics, events, topology, alert history and workload history.
Explicit local log consent adds an eighth read. Every family has a durable
reservation before I/O, at most eight records and an 8-KiB projection limit.
Combined input/output retain their existing independent 64-KiB bounds. The service
does not follow cursors or retry uncertain generation. Explanation remains one read.

The local repository ports bind source, provider, exact resource and the half-open
selected window (at most 24 hours). Current inventory, session, source version,
restore generation and read authority are checked around collection and again
before counting, dispatch, publication and citation reads. History never grants
new authority and cannot be requested through browser-supplied tool names.

Readable empty history removes the corresponding unread gap without inventing a
fact. Unsupported/unavailable reads retain the unread gap. Historical unavailable
workload samples carry null counts, not zero replicas. Actual occurrence/sample
times remain distinct from freshly fenced read time. Alert transitions contain
only closed states. Workload samples contain only bounded desired/ready counts.
No rule prose, log message, native handle or arbitrary explanation enters these fields.

The report shows qualified workload shortfall and any later recovery as contrary
evidence. Correlation does not prove cause. Both languages label the families,
times, quality and counts. Opening a report witness performs the existing current
authorized citation read, then focuses its timeline row. The report and answer
share the retained output cap. Owner loss, revocation, cancellation, expiration,
source changes and leaving clear volatile text; history reports are not persisted.

Required hosted checks cover reservation order, empty versus unread families,
recovery, stale/partial/unavailable values, mismatched references, duplicate IDs,
unsafe counts, wrong time basis, lost authority and both desktop languages.
Real PostgreSQL history/migration checks, provider/install gates, concurrency,
the chosen processor and browser journeys remain required. This candidate does
not replace the held-out quality, security review, pilot/soak or public-consumer
gates needed for stable version 2.

## فارسی

بررسی با انتخاب صریح کاربر شروع می‌شود. تاریخچهٔ هشدار و بار کاری هرکدام
رزرو و مجوز تازه دارند؛ دادهٔ خالیِ قابل‌خواندن با دادهٔ خوانده‌نشده فرق دارد.
زمان وقوع یا نمونه‌برداری عوض نمی‌شود. شمارشِ ناموجود با صفر نمایش داده نمی‌شود.
کمبود replica فقط یک احتمال است؛ بهبود بعدی شاهد مخالف است. بازکردن شاهد،
مجوز جاری را دوباره می‌سنجد و ردیف همان شاهد را فوکوس می‌کند. متن خصوصی
در مرورگر یا پایگاه داده ذخیره نمی‌شود. تست و ساخت فقط روی GitHub Actions.
پذیرش کل گام ۶ و نسخهٔ پایدار ۲ همچنان رسید مستقل می‌خواهد.
