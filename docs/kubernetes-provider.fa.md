# مشاهده Kubernetes

[English](kubernetes-provider.md) · [پیکربندی](configuration.md) · [مرز فراهم‌کننده](adr/0002-provider-neutral-boundary.md)

آداپتر گام ۵ از بسته‌های رسمی `client-go`، `api`، `apimachinery` و `metrics`
با نسخه ثابت `v0.37.1` استفاده می‌کند. APIهای core، apps، batch، networking و
discovery نسخه `v1` هستند. Metrics API اختیاری، نسخه `metrics.k8s.io/v1beta1`
دارد؛ همان استثنای پذیرفته‌شده ADR 0002. ساختار خام SDK از آداپتر بیرون نمی‌رود.

## دامنه و راه‌اندازی

منبع پیش‌فرض خاموش است. روشن کردن آن فهرست صریح namespace می‌خواهد:

```dotenv
STACKPLORE_KUBERNETES_ENABLED=true
STACKPLORE_KUBERNETES_NAMESPACES=shop,ops
STACKPLORE_KUBERNETES_CLUSTER_OBSERVATION=false
```

نام باید DNS label معتبر Kubernetes باشد: حرف کوچک، رقم و خط تیره در میانه؛
۱ تا ۶۳ نویسه. فهرست ۱ تا ۶۴ عضو دارد؛ با ویرگول و بدون فاصله. خواننده تنظیمات
نام‌ها را مرتب و تکرار را حذف می‌کند. سازنده آداپتر نیز مرز را می‌سنجد و ورودی
تکراری را رد می‌کند.

نشانی درون خوشه ثابت است: `https://kubernetes.default.svc`.
`STACKPLORE_KUBERNETES_TOKEN_FILE` و `STACKPLORE_KUBERNETES_CA_FILE` فقط مسیر
مطلق فایل زیر اختیار اپراتور هستند. پیش‌فرض، فایل‌های `token` و `ca.crt` حساب
سرویس در `/var/run/secrets/kubernetes.io/serviceaccount/` است. خواندن توکن سقف
۱۶ KiB و CA سقف ۱ MiB دارد. TLS باید نام سرویس را تأیید کند. مرورگر حق انتخاب
نشانی، فایل، اعتبارنامه یا افزونه اجرای احراز هویت ندارد.

توکن projected حساب سرویس در درخواست عادی، حداکثر هر ۶۰ ثانیه از فایل محدود
دوباره خوانده می‌شود. پاسخ `401` آن cache را باطل می‌کند. خواندن فقط یک بار
می‌تواند با توکن تازه و متفاوت تکرار شود. نوشتن در این مسیر تکرار نمی‌شود.
شکست نوسازی، درخواست را رد می‌کند؛ توکن کهنه جایگزین آن نمی‌شود. پایان کار،
اتصال‌های بیکار transport و ارجاع توکن cache را می‌بندد.

حالت namespace فقط دامنه‌های فهرست‌شده را می‌خواند. فهرست namespaceها، nodeها
یا همه دامنه‌های خوشه خوانده نمی‌شود. هویت خوشه، شیء namespace و داده node در
این حالت مشاهده نشده‌اند. منبع پوشش جزئی با کد ثابت پروفایل دارد؛ منبع سالم
خوشه ساخته نمی‌شود. نوع غیرفعال در پوشش اختیار نمی‌آید؛ در نتیجه داده قدیمی
خوشه پس از محدود شدن پروفایل از cache قابل نمایش نمی‌ماند.

با `STACKPLORE_KUBERNETES_CLUSTER_OBSERVATION=true`، فهرست nodeها و هر
namespace مجاز با نام خوانده می‌شود. سایر فهرست‌ها همچنان فقط در دامنه مجاز
هستند. شناسه namespace از UID است؛ حذف و ساخت دوباره، هویت را عوض می‌کند.
دسترسی به API دلیل سلامت خوشه نیست؛ سلامت خوشه بدون شاهد، نامعلوم می‌ماند.

## kubeconfig نصب‌شده و context

برای مشاهدهٔ خوشه از بیرون، اپراتور نمایهٔ نصب‌شدهٔ فقط‌خواندنی را می‌دهد:

```dotenv
STACKPLORE_KUBERNETES_ENABLED=true
STACKPLORE_KUBERNETES_NAMESPACES=shop,ops
STACKPLORE_KUBERNETES_ACCESS_PROFILE=kubernetes_kubeconfig
STACKPLORE_KUBERNETES_KUBECONFIG_FILE=/run/stackplore/kubernetes/config.yaml
STACKPLORE_KUBERNETES_REFERENCE_ROOTS=/run/stackplore/kubernetes
STACKPLORE_KUBERNETES_CONTEXT=observed-cluster
```

فایل با `apiVersion: v1` و `kind: Config` است. cluster یک `server` با origin
دقیق HTTPS و ارجاع فایل `certificate-authority` دارد. user فقط `tokenFile` یا
جفت `client-certificate` و `client-key` دارد. مسیر نسبی کنار kubeconfig حل می‌شود؛
مسیر مطلق هم باید زیر ریشهٔ مجاز باشد. جداکنندهٔ چند ریشه در Linux `:` و در
Windows `;` است. اگر context صریح داده نشود، `current-context` انتخاب می‌شود.
namespace داخل context اختیاری است؛ فهرست namespace پیکربندی دامنه را تعیین می‌کند.

سقف فایل و CA/گواهی/کلید هرکدام ۱ MiB و توکن ۱۶ KiB است. تمام contextها از نظر
inline secret/CA، افزونهٔ اجرا یا auth-provider، basic، proxy، impersonation،
تغییر نام TLS، TLS ناامن، extension، کلید ناشناخته و چند سند بررسی و رد می‌شوند.
تمام symlinkها، فایل ویژه و ارجاع بیرون ریشه‌ها رد می‌شوند. مسیر پیش‌فرض اعتبارنامه
یا fallback محیط اجرا نمی‌شود. TLS نام endpoint و CA انتخاب‌شده را می‌سنجد.
مرز ثابت endpoint خواندن و namespace در transport باقی می‌ماند.

پایگاه فقط شناسهٔ نمایه و digest اختیار را نگه می‌دارد؛ متن فایل، endpoint، مسیر
یا اعتبارنامه ذخیره نمی‌شود. تغییر نمایه، context یا پیکربندی بی‌راز، cache قدیم
را می‌بندد تا جمع‌آوری تازه منتشر شود. تغییر kubeconfig، CA یا گواهی به restart
نیاز دارد؛ token-file با قاعدهٔ محدود قبلی نوسازی می‌شود. این نمایه
`KUBERNETES_WRITE_ENABLED=true` را رد می‌کند. writer درون خوشه مجوز جدا می‌خواهد.
چارت بسته‌بندی‌شده همچنان نمایهٔ درون خوشه نصب می‌کند؛ mount بیرونی با اپراتور است.

## مجوز Helm

```yaml
api:
  kubernetes:
    enabled: true
    namespaces: [shop, ops]
    clusterObservation: false
```

namespaceهای هدف را پیش از نصب بسازید. نصب‌کننده باید بتواند Role و RoleBinding
چارت را در هر دامنه هدف بسازد. binding به حساب سرویس release در namespace نصب
اشاره دارد. مشاهده فقط `get` و `list` دارد؛ `pods/log` فقط `get`. خواندن شیء
Secret یا ConfigMap، نوشتن، `exec`، attach، proxy و مجوز wildcard داده نمی‌شود.

گزینه خوشه، ClusterRole و ClusterRoleBinding جدا می‌سازد: `get/list` برای node
و `get` برای namespaceهای مقید به `resourceNames`. مجوز تغییر زیرساخت اضافه
نمی‌شود. چارت روشن با فهرست خالی رد می‌شود. چارت، namespace هدف را نمی‌سازد و
برای دامنه بیرون فهرست مجوز نمی‌دهد.

## موجودی، سلامت و رابطه

کشف شامل Pod و container عادی/init؛ Service؛ PVC؛ Deployment، StatefulSet،
DaemonSet، ReplicaSet، Job و CronJob؛ Ingress و EndpointSlice است. ارجاع
ConfigMap و Secret فقط از بخش امن تعریف Pod و workload می‌آید؛ از جمله template
با صفر replica، volume نوع projected، init container و ارجاع image-pull.
بدنه آن اشیا خوانده نمی‌شود.

هویت شیء مشاهده‌شده از منبع، محیط، نوع، namespace و UID اصلی ساخته می‌شود.
container، نام خود را به UID همان Pod می‌افزاید. مدخل ConfigMap و Secret فقط نام
ارجاع‌شده در تعریف مجاز است؛ کلید مرجع قطعی دارد و وجود، UID، محتوا یا ساخت
دوباره شیء را ثابت نمی‌کند. جزئیات امن، وضعیت محدود container، آمادگی، شمار
راه‌اندازی دوباره، درخواست/سقف CPU و حافظه و شرط‌های شناخته‌شده را دارد. مقدار
محیط، فرمان، پیام خام، label دلخواه و مقدار Secret جایی در این مدل ندارند.
وضعیت اجرای نامعلوم، شاهد صفر بودن restart یا شکست readiness نیست. در rollout
ممکن است شمار ready از desired بیشتر باشد؛ این حالت شاهد replica را حذف نمی‌کند.

رابطه‌ها از مالکیت مشاهده‌شده، selector سرویس، مسیر Ingress، هدف EndpointSlice،
mount و ارجاع پیکربندی می‌آیند. controller سفارشی ناشناخته، موجودی دیگر را دور
نمی‌ریزد. وابستگی annotation این قالب دقیق را دارد:

```yaml
metadata:
  annotations:
    stackplore.io/depends-on: Service/database,Deployment/worker
```

فهرست با ویرگول از `kind/name`، فقط در **همان namespace** است. سقف ۲۰۴۸ بایت و
۳۲ عضو دارد. بزرگی حرف kind مهم نیست. نوع‌های مجاز: Pod، Service، Deployment،
StatefulSet، DaemonSet، ReplicaSet، Job، CronJob، Ingress، PersistentVolumeClaim،
ConfigMap و Secret. دو نام `persistent_volume_claim` و `config_map` نیز پذیرفته
می‌شوند. نام هدف باید DNS subdomain معتبر Kubernetes باشد. عضو نامعتبر نادیده
می‌ماند؛ فهرست بیش از سقف، کامل کنار می‌رود. یال فقط وقتی ساخته می‌شود که هدف
در موجودی مجاز حاضر باشد. یال annotation شاهد صریح تعریف است؛ وجود اتصال زنده
شبکه را ثابت نمی‌کند.

## دسترسی جزئی و اختیار cache

هر جفت namespace/نوع، نتیجه جدا دارد. رد شدن Ingress، Pod موفق را حذف نمی‌کند.
`401/403` همان بخش را denied می‌کند؛ API اختیاری غایب unsupported است. timeout
و سقف صفحه، عضو یا بایت هیچ‌گاه پایان کامل فهرست نیستند. شکست صفحه بعد، عضوهای
امن قبلی را به صورت جزئی نگه می‌دارد؛ پاسخ denied یا unsupported، عضوهای فعلی
آن بخش را کنار می‌گذارد. continuation تکراری، خواندن را قطع می‌کند.

فقط بخش available و complete می‌تواند هویت قدیمی دیده‌نشده را بازنشسته کند.
پس فهرست خالی و کامل Pod، Pod حذف‌شده را بازنشسته می‌کند حتی اگر Ingress رد
شده باشد. اختیار denied، unsupported یا حذف‌شده داده قدیمی را نمایش نمی‌دهد.
بخش مجاز unavailable می‌تواند شاهد کهنه را نگه دارد. اگر هیچ بخش قابل استفاده
نیست و رد دسترسی فعلی معلوم است، منبع permission denied برمی‌گرداند؛ آن را به
قطعی عمومی تبدیل نمی‌کند. تغییر دامنه اپراتور تا انتشار معتبر اختیار تازه در
پایگاه داده حصار دارد؛ شکست نخستین جمع‌آوری، داده پروفایل قبلی را باز نمی‌کند.

نام ارجاع به فهرست Pod و workload والد بسته است. اگر یک نوع والد در namespace
رد یا unsupported شود، هر دو بخش نام ارجاع ConfigMap و Secret آنجا پنهان
می‌شوند. این قاعده محتاط، نام قدیمی را پس از سلب مجوز والد نگه نمی‌دارد.
شاهد Pod و workload مجاز دیگر همچنان قابل استفاده است.

## لاگ، رویداد و سنجه

لاگ تاریخی، انتخاب دقیق container و نمونه فعلی/قبلی را دارد. `sinceTime`، سقف
بایت، timestamp و بدون follow خوانده می‌شود. `tailLines` بین صفحه‌ها عوض نمی‌شود.
پنجره خواندن به نخستین ۱۰۰۰ خط و سقف بایت درخواست، میان حداکثر ۶۴ هدف container
گسترش‌یافته محدود است. صفحه HTTP بیش از ۱۰۰ خط ندارد. برش نتیجه، partial است؛
تاریخچه خالی و کامل نیست. پوشاندن داده حساس پیش از سنجش نهایی سقف بایت پاسخ است.
cursor به منبع، هدف، فعلی/قبلی بودن، بازه زمان و پنجره مشاهده‌شده بسته است.
پنجره عوض شود، cursor قدیمی رد می‌شود؛ خواندن تاریخچه را از نو آغاز کنید.
پیش و پس از هر خواندن لاگ، Pod دقیق همان namespace دوباره خوانده می‌شود و باید
UID معتبر موجودی را داشته باشد. حذف، ساخت دوباره، سلب مجوز یا هویت نامعلوم،
خطوط همان هدف را حذف می‌کند. این بررسی‌ها همان مهلت و سقف بایت اصلی را دارند.

رویداد از core Events API همان namespace و شاهد UID/زمان/شمار می‌آید. پیام
رویداد از مرز متن امن لاگ می‌گذرد. سقف جمع‌آوری ۱۰۰۰ رویداد است. قابلیت watch
یا دنبال کردن مستقیم stream لاگ ادعا نمی‌شود.

مصرف CPU و حافظه از Resource Metrics API است. نبود API با `404`، unsupported
می‌دهد؛ نمونه صفر ساخته نمی‌شود. رد مجوز هر namespace جدا می‌ماند. هر رد مجوز
مصرف، با احتیاط کل درگاه سنجه فعلی/تاریخی، fallback از cache و نمونه تنظیم منابع
را می‌بندد. موجودی و metadata امن تنظیم منابع باز می‌ماند. این کار، نمایش مصرف
قدیمی پس از تغییر مجوز را می‌بندد؛ اختیار جدا برای هر نام سنجه منتشر نمی‌شود. درخواست/سقف
تعریف‌شده هر container و شمار restart مشاهده‌شده، بدون این API هم قابل استفاده
است. جمع ساده درخواست containerها، درخواست مؤثر زمان‌بندی Pod معرفی نمی‌شود؛
قواعد init و sidecar آن را کافی نمی‌دانند. مقدار نرسیده، نامعلوم می‌ماند.

زمان مصرف بیش از یک دقیقه جلوتر از جمع‌آوری رد می‌شود. بازه کسری SDK برای
قرارداد HTTP به نزدیک‌ترین ثانیه کامل گرد می‌شود؛ زمان نمونه دقت میکروثانیه دارد.
مصرف، UID معتبر ثابت حین مشاهده و زمان ساخت معلوم می‌خواهد؛ کل پنجره نمونه
باید در عمر همان Pod باشد. نمونه cache از Pod قبلی هم‌نام به Pod تازه نمی‌چسبد.
هویت یا عمر نامعلوم، مصرف را پنهان می‌کند؛ شاهد تنظیم منابع همچنان قابل استفاده است.

## سقف و مدرک

سازنده حداکثر ۱۰هزار عضو و ۳۲ MiB داده پاسخ می‌پذیرد؛ اتصال تولید ۱۰هزار و
۸ MiB دارد. عضوهای خام و منابع گسترش‌یافته دو سقف جدا دارند. سقف بایت میان کل
جمع‌آوری مشترک است؛ کل کار حداکثر ۳۰ ثانیه و هر صفحه حداکثر ۵ ثانیه. مهلت
کوتاه‌تر فراخواننده مقدم است. هر فهرست حداکثر ۱۰۰ صفحه، هر صفحه تا ۵۰۰ عضو دارد.
transport، redirect، فشرده‌سازی نامنتظر و تأخیر تکرار پنهان را رد می‌کند. محدودیت
نرخ میان clientها مشترک است. مسیر محصول به exec، proxy دلخواه یا Secret read
در SDK راه ندارد.

تست محلی، درخواست واقعی client رسمی روی پاسخ کنترل‌شده، مرز scope، دسترسی جزئی،
صفحه‌بندی، لغو، سقف‌ها، نوسازی توکن، metadata، cursor لاگ و شاهد replica را پوشش
می‌دهد. lint/render چارت و رد فهرست خالی هم محلی آزموده شد. این‌ها مدرک خوشه
زنده نیستند.

کار اجباری `kubernetes-provider` در CI، kind مستقل خود را می‌سازد: نسخه
`v0.33.0` و Kubernetes `v1.37.0` با hash ثابت binary و image. حساب سرویس تازه
در یک namespace مجاز و در دیگری رد است؛ یک نوع اختیاری رد می‌شود. لاگ فعلی/قبلی،
رویداد، نبود Metrics API و حذف/جمع‌آوری دوباره سنجیده می‌شوند. transport مشاهده
فقط CA دارد؛ گواهی مدیر fixture نمی‌تواند محدودیت حساب سرویس را دور بزند. نبود
تنظیم هدف، کار اجباری را fail می‌کند و skip نمی‌شود.

آزمون مستقل بعدی، metrics-server نسخه `v0.9.0` را از manifest رسمی با checksum
ثابت و image با digest ثابت نصب می‌کند؛ نمونه تازه CPU/حافظه و رد namespace
را می‌سنجد. `scripts/install-test-metrics-server.sh` فقط با پرچم خوشه دورریختنی
و context دقیق `kind-stackplore-proof-*` متعلق به آزمون کار می‌کند. فقط همین
fixture برای گواهی self-signed kubelet در kind، `--kubelet-insecure-tls` دارد؛
مشاهده همچنان CA سرور API را بررسی می‌کند.
مشاهده‌گر دو namespace ثابت می‌کند رد دسترسی، کل درگاه سنجه را می‌بندد.
مشاهده‌گر فقط namespace مجاز، با همان حساب سرویس، شاهد تنظیم منابع در نبود API
و مصرف مثبت در حضور API را می‌سنجد.

**مدرک هدف فعلی، ۲۰۲۶-۱۰-۱۷:** هر دو تست نهایی روی Kubernetes `1.37.0` دورریختنی،
kind `0.33.0`، Docker Engine `29.1.3` و Go `1.26.5` در WSL محلی پاس شدند.
تست API غایب `7.06s` و تست مثبت API سنجه `18.84s` طول کشید. هر دو یک binary
نهایی لینوکس با SHA-256 زیر را اجرا کردند:
`b9614bc6a63e6eeb6a5d9c9932d2f49f5fad1f9ab0b3037e709377fef9f6753c`.
خوشه نام‌دار و kubeconfig موقت آن پاک شدند؛ container متعلق به kind باقی نماند.
نسخه‌های دیگر Kubernetes و اجرای CI راه‌دور در این مدرک محلی نتیجه ندارند.

دریافت نخست image سنجه داخل node محلی با `403` رجیستری شکست خورد. اجرای نهایی،
index رسمی، manifest معماری بومی، config و ۱۴ لایه همان image را از شبکه میزبان
گرفت؛ hash هر descriptor چک شد و فقط به node دورریختنی وارد شد. digest ثابت
image حفظ شد؛ تنظیم daemon عوض نشد. رسید شکست pull/import جدا از نتیجه پاس
ماند. رسیدهای محلی:
`.tmp/stage5-kube-final-real.log`، `.tmp/stage5-kube-final-build.log`،
`.tmp/stage5-kube-final-uid-tests.log`، `.tmp/stage5-kube-final-uid-vet.log`،
`.tmp/stage5-kube-helm-final.log` و `.tmp/stage5-kind-cleanup.log`.
دروازه‌های مرحله در [برنامه فعال شواهد](implementation-plan.md) ثبت می‌شوند.

منبع اصلی:
[client-go v0.37.1](https://github.com/kubernetes/client-go/tree/v0.37.1)،
[metrics v0.37.1](https://github.com/kubernetes/metrics/tree/v0.37.1)،
[نسخه و hashهای kind v0.33.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)،
[metrics-server v0.9.0](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.9.0).
