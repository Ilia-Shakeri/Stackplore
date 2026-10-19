# Stackplore artwork and interface palette

The project owner supplied the three source logos on 2026-10-19. Source files
remain unchanged outside the repository. The checked-in derivatives use the
provided artwork; no replacement symbol or typeface was invented.

| Use                                            | Asset                                                                                                                      | Delivery                                                                                            |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| Navigation, mobile menu, setup and login shell | [Transparent mark](../apps/web/public/brand/stackplore-mark-192.webp)                                                      | 96/192 WebP density pair; explicit dimensions; decorative beside the named home link                |
| About, light theme                             | [Light introduction](../apps/web/public/brand/stackplore-intro-light.webp)                                                 | Original white-background composition, proportional 960×640 WebP                                    |
| About, dark theme                              | [Dark introduction](../apps/web/public/brand/stackplore-intro-dark.webp)                                                   | Original neon composition, proportional 960×640 WebP                                                |
| Browser tab and public offline shell           | [64px PNG](../apps/web/public/brand/stackplore-favicon-64.png)                                                             | Transparent 32/64px variants; no external resource request                                          |
| Installed application                          | [192px PNG](../apps/web/public/brand/stackplore-pwa-192.png), [512px PNG](../apps/web/public/brand/stackplore-pwa-512.png) | Square navy canvas, original proportions                                                            |
| Maskable application icon                      | [512px maskable PNG](../apps/web/public/brand/stackplore-maskable-512.png)                                                 | Separate artwork inset within the central safe circle                                               |
| Apple home screen                              | [180px PNG](../apps/web/public/brand/stackplore-apple-touch.png)                                                           | Opaque navy canvas                                                                                  |
| Documentation and larger mark                  | [512px WebP](../apps/web/public/brand/stackplore-mark-512.webp)                                                            | Transparent mark                                                                                    |
| Share artwork                                  | [1200×630 JPEG](../apps/web/public/brand/stackplore-social-preview.jpg)                                                    | Downloadable public artwork; deployments must use their own absolute public URL for social metadata |

The three originals total **3,629,894 bytes**. All twelve derivatives together
total **525,385 bytes**, an **85.5% reduction** despite additional sizes and
formats. The ordinary navigation mark is **4,092 bytes** at 96px; introduction
images are **31,300/48,420 bytes**. Only the selected introduction theme loads.
[assets.json](../apps/web/public/brand/assets.json) records source filenames,
dimensions, exact SHA-256 digests, encoding and output byte counts.

Processing cropped only empty transparent margins of the supplied cutout,
resized proportionally with Lanczos, retained its alpha channel for marks and
favicons, and encoded WebP at quality 85/88 or optimized PNG/JPEG. Source PNGs
are not duplicated in the application bundle. Standard application builds use
the prepared assets and need no image-processing runtime.

The shared light/dark tokens use navy, blue and cyan from the logos. Light
primary is `#075ad4` with white text; dark primary is `#43bcff` with `#071424`
text. Dark background is `#06111f`; light background is `#f4f8fd`. Orange is a
sparse accent. Health, warning and danger colors retain semantic meaning and
text/icons; they are not decorative brand colors. The selected theme, startup
script, browser theme color and offline shell use the same palette.

Keep focus contrast, keyboard operation, reduced motion, RTL layout and
technical-text isolation. Do not animate the logo or add glow behind reading
surfaces. Give decorative images empty alternative text when adjacent text or
the link already supplies the name; the introduction is decorative beside the
visible product name and translated page heading.

The service worker still caches only four public shell assets: offline HTML,
CSS, its fixed script and the 64px logo. Application pages, conversations,
authentication, infrastructure responses and private streams remain network
only. Replacing branding does not expand offline data storage.

The [local candidate receipt](evidence/2.0.0-alpha.0/README.md) records the full
local gate, fourteen actual browser cases, both offline theme overrides and
thirty-two screenshots. Desktop light/dark and Persian mobile dark layouts were
visually checked. These checks cover the supplied branding and existing product
journeys; helper runtime acceptance remains separate.

## فارسی

سه لوگو از صاحب پروژه؛ اصل فایل‌ها دست نخورده. نشان شفاف در ناوبری و قاب
ورود؛ طرح سفید و نئون در معرفی، برابر تم؛ نشان کوچک در تب، نصب و صفحهٔ آفلاین.
منو، نام، نسبت تصویر، فضای ثابت و حق انتخاب تم حفظ شوند.

اصل‌ها **۳٬۶۲۹٬۸۹۴ بایت**؛ دوازده خروجی **۵۲۵٬۳۸۵ بایت**؛ کاهش **۸۵٫۵٪**.
نشان روزمره فقط **۴٬۰۹۲ بایت**. حاشیهٔ خالی شفاف بریده شد؛ نسبت و شفافیت حفظ؛
نسخهٔ PNG بزرگ در بسته تکرار نشود. فهرست hash و حجم در `assets.json` است.

سرمه‌ای، آبی و فیروزه‌ای پایه؛ نارنجی کم. رنگ خطر و سلامت، معنای خود را نگه
دارند. کنتراست، focus، فارسی/انگلیسی و RTL لازم؛ حرکت تازه برای لوگو افزوده نشود.
cache آفلاین همان چهار فایل عمومی بماند؛ هیچ دادهٔ شخصی یا زیرساخت ذخیره نشود.
