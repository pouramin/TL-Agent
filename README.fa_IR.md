[English](./README.md) | [فارسی](./README.fa_IR.md)

<p align="center">
  <img src="./media/tl-agent-logo.svg" width="300" alt="TL Agent">
</p>

<h1 align="center">TL Agent</h1>

<p align="center">
  یک محیط مستقل و لوکال برای کار با Coding Agent؛ بدون نیاز به IDE.
</p>

<p align="center">
  <a href="https://github.com/pouramin/TL-Agent/releases"><img src="https://img.shields.io/github/v/release/pouramin/TL-Agent?sort=semver" alt="Release"></a>
  <a href="https://github.com/pouramin/TL-Agent/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/pouramin/TL-Agent/ci.yml?branch=main&label=CI" alt="CI"></a>
  <a href="https://www.npmjs.com/package/tl-agent"><img src="https://img.shields.io/npm/v/tl-agent" alt="npm"></a>
  <a href="https://github.com/pouramin/TL-Agent/releases"><img src="https://img.shields.io/github/downloads/pouramin/TL-Agent/total" alt="Downloads"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/pouramin/TL-Agent" alt="License"></a>
</p>

**TL Agent** یک Workspace اختصاصی در مرورگر برای کار Coding Agentها روی پروژه‌های لوکال است. می‌توانید فولدر پروژه را باز کنید، Agent و Model را انتخاب کنید، فایل ضمیمه کنید، فعالیت Toolها و تغییرات را ببینید، Sessionها را مدیریت کنید و Permissionها را پاسخ بدهید؛ درحالی‌که Workspace روی کامپیوتر خودتان باقی می‌ماند.

برای استفاده‌ی معمول نیازی به VS Code، JetBrains، Cursor، Docker، Backend ابری TL Agent یا Database جداگانه نیست.

## شروع سریع

### اجرا با یک دستور

اگر Node.js و npm نصب هستند، داخل فولدر پروژه‌ای که می‌خواهید روی آن کار کنید این دستور را اجرا کنید:

```bash
npx --yes tl-agent
```

پکیج npm فقط یک Launcher سبک است. سیستم‌عامل و معماری را تشخیص می‌دهد، Release رسمی و متناظر TL Agent را از GitHub دانلود می‌کند، SHA-256 آن را بررسی می‌کند، فایل را به‌صورت محلی Cache می‌کند و TL Agent را با فولدر فعلی به‌عنوان Project باز می‌کند.

برای جلوگیری از بازشدن خودکار مرورگر:

```bash
npx --yes tl-agent --no-browser
```

نسخه‌های آزمایشی همچنان می‌توانند با Channel مشخص اجرا شوند، برای مثال:

```bash
npx --yes tl-agent@alpha
```

### نسخه‌ی Portable

برای نسخه‌ی Portable نیازی به Node.js نیست. فایل مناسب سیستم خود را از **[GitHub Releases](https://github.com/pouramin/TL-Agent/releases)** دانلود و Extract کنید، سپس اجرا کنید:

```text
Windows:  tl-agent.exe
Linux:    ./tl-agent
macOS:    ./tl-agent
```

Runtime لوکال سازگار از قبل داخل Release قرار دارد.

## قابلیت‌ها

- **Workspace مستقل و لوکال** — رابط اختصاصی Coding Agent در مرورگر، بدون IDE.
- **انتخاب مستقیم Project** — بازکردن فولدر با Folder Picker خود سیستم‌عامل.
- **انتخاب Agent و Model** — تغییر Agent و مدل‌های Providerها از داخل Composer.
- **Custom Provider** — اتصال Endpointهای سازگار با OpenAI، OpenAI Responses و Anthropic با Credential خود کاربر.
- **File attachment** — ارسال تصویر، PDF و فایل‌های متنی/کد؛ همراه با Multi-select، Drag & Drop و Paste از Clipboard.
- **نمایش زنده‌ی فعالیت Agent** — نمایش Reasoning و Toolها همراه با وضعیت Run.
- **Permission و Question** — Allow یک‌باره، ذخیره‌ی Ruleهای قابل‌تکرار در صورت پشتیبانی، Reject و پاسخ به سؤال‌های تعاملی.
- **Stop و Recovery** — توقف Run فعال و بازیابی Sessionهای گیرکرده یا خطاهای Retryable.
- **مدیریت Session** — ساخت، ادامه، تغییر نام، حذف و جابه‌جایی Sessionها بین Projectهای اخیر.
- **Project Usage** — نمایش مصرف هر Turn و مجموع Project شامل Token، Request، Time، Reasoning و Cache.
- **Changes panel** — مشاهده‌ی فایل‌های تغییرکرده، تعداد خطوط اضافه/حذف‌شده و Patch.
- **Project Workspace داخلی** — File Explorer قابل‌نوشتن و Monaco Editor لوکال و Lazy-loaded با ویرایش چندتب، Find/Replace، Multi-cursor، Save/Create/Rename/Delete و هماهنگی با تغییرات خارجی فایل.
- **Project Search** — جست‌وجوی سریع متن در کل Project با Include/Exclude و بازکردن مستقیم نتیجه در Editor.
- **Terminal داخلی** — اجرای Command در Scope پروژه، تاریخچه‌ی خروجی، Stop و پایان Process tree.
- **Live Preview** — Preview لوکال برای Static یا Node dev server در پنجره‌ی قابل‌جابجایی و تغییر اندازه.
- **تنظیمات ظاهر و Editor** — حالت System، Dark و Light به‌همراه Editor theme و Font جداگانه برای UI، Code و Terminal.
- **معماری Local-first** — اجرای Loopback-only، رمز تصادفی Backend در هر اجرا، کنترل Origin و CSP محدودکننده.
- **بدون Cloud یا Telemetry اختصاصی TL Agent** — ترافیک Model براساس Provider و Runtime انتخاب‌شده‌ی کاربر انجام می‌شود و از زیرساخت TL Agent عبور نمی‌کند.

## معماری

```text
Browser workspace
    │ فقط localhost
    ▼
TL Agent launcher (Go)
    │
    ├─ TL Agent provider/model registry
    ├─ project files / search / terminal / preview
    │
    └─ runtime adapter محلی و احراز‌شده
            ▼
        Local agent runtime
            ├─ agents / sessions / tools
            ├─ permissions / questions / live events
            └─ provider execution / model inference
```

TL Agent مالک لایه‌ی محصول است: Workspace، رابط کاربری، Launcher محلی، تعریف Provider و Model، تجربه‌ی Project و Session، Recovery و Release packaging.

تعریف Custom Providerها داخل State محلی خود TL Agent ذخیره می‌شود و Launcher آن‌ها را برای Runtime فعال ترجمه می‌کند. Credentialها فعلاً به Credential Store محلی Runtime سپرده می‌شوند و داخل Provider Registry خود TL Agent ذخیره نمی‌شوند.

Runtime به‌عنوان یک لایه‌ی زیرساختی جدا پشت این مرز قرار می‌گیرد.

Project انتخاب‌شده روی سیستم کاربر باقی می‌ماند و TL Agent ترافیک Model را از زیرساخت خودش عبور نمی‌دهد.

## مرز Runtime

مرورگر و رابط محصول TL Agent به قراردادهای خود TL Agent وابسته‌اند، نه به API اختصاصی یک Engine. ترافیک Runtime مرورگر فقط از مرز محلی `/runtime/*` عبور می‌کند و تعریف Provider/Model، فایل‌های Project، Search، Terminal، Preview و بخش‌های اصلی Workspace در مالکیت TL Agent هستند.

نسخه‌ی Stable فعلی، **Kilo Code 7.6.2** را به‌عنوان Agent Engine شخص ثالث و تست‌شده Bundle می‌کند. این Engine یک جزئیات پیاده‌سازی پشت Runtime Adapter است و هویت عمومی محصول به آن وابسته نیست. جزئیات سازگاری Engine در [`docs/KILO_API_CONTRACT.md`](./docs/KILO_API_CONTRACT.md) و Attribution لازم در [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) نگهداری می‌شود.

CI همین Engine پین‌شده را از طریق مرز عمومی Runtime خود TL Agent برای Project routing، APIهای Agent/Provider/Session، Async Prompt، Live events، Permission، Provider configuration، اجرای Tool و Write واقعی روی فایل تست می‌کند.

## نسخه‌های قابل دانلود

| سیستم‌عامل | معماری |
| --- | --- |
| Windows | x64 |
| Linux | x64، ARM64 |
| macOS | Intel x64، Apple Silicon ARM64 |

## ساختار فایل Release

```text
tl-agent/
├─ tl-agent[.exe]
├─ bin/
│  └─ kilo[.exe]
├─ LICENSE
├─ THIRD_PARTY_NOTICES.md
└─ third_party/
   ├─ KILO_LICENSE.txt
   ├─ MONACO_LICENSE.txt
   └─ MONACO_THIRD_PARTY_NOTICES.txt
```

## اجرا از سورس

نیازمندی‌های Development:

- Go 1.23 یا جدیدتر
- Runtime binary سازگار در `PATH`، کنار Launcher یا مشخص‌شده به‌صورت دستی

```bash
go run ./cmd/launcher
```

بازکردن یک Project مشخص:

```bash
go run ./cmd/launcher --project /path/to/project
```

استفاده از Runtime binary مشخص:

```bash
go run ./cmd/launcher --runtime-bin /path/to/runtime
```

برای جلوگیری از بازشدن خودکار مرورگر از `--no-browser` استفاده کنید.

## قانون زیرساخت صفر

TL Agent طوری طراحی شده که نگهدارنده برای اجرای پروژه نیازی به پرداخت هزینه‌ی VPS، Hosting، Database، API Gateway، Model inference یا Telemetry backend نداشته باشد. سورس، Issueها، CI، Releaseها، فایل‌های دانلودی و Launcher سبک npm از زیرساخت GitHub/npm توزیع می‌شوند.

هزینه‌ی احتمالی استفاده از Model مستقیماً بین کاربر و Provider انتخاب‌شده‌ی اوست.

## مدل امنیتی

Launcher:

1. رابط را فقط روی Loopback اجرا می‌کند؛
2. Runtime لوکال را با رمز تصادفی در هر اجرا بالا می‌آورد؛
3. رمز Backend را سمت Server نگه می‌دارد؛
4. Project انتخاب‌شده را فقط به‌صورت محلی Route می‌کند؛
5. درخواست‌های Cross-origin را رد می‌کند؛
6. و رابط را با Content Security Policy محدودکننده سرو می‌کند.

Runtime در صورت داشتن Permission می‌تواند فایل‌ها را بخواند، بنویسد و Command اجرا کند. TL Agent را فقط روی سیستم و Projectهایی اجرا کنید که به آن‌ها اعتماد دارید.

## وضعیت پروژه

TL Agent خط پایدار Production را روی `main` و توسعه‌ی آزمایشی را روی `dev` نگه می‌دارد. نسخه‌ی Stable فقط بعد از عبور از CI خودکار و تست دستی روی یک سیستم واقعی Windows ارتقا داده می‌شود. مسیر اصلی که پیش از Promotion بررسی می‌شود شامل این زنجیره است:

```text
TL Agent UI
→ local agent runtime
→ selected model
→ tool call
→ permission
→ local file write
→ final assistant response
```

نسخه‌های آزمایشی روی Channelهای prerelease جدا ادامه پیدا می‌کنند، بدون اینکه مسیر Stable با dist-tag `latest` تغییر کند.

## لایسنس و Attribution

کد Launcher و UI پروژه‌ی TL Agent تحت لایسنس MIT منتشر شده است. Runtime فعلی Kilo Code نیز MIT است و به‌عنوان یک پروژه‌ی مستقل Upstream باقی می‌ماند. Releaseهایی که آن را Bundle می‌کنند، License Notice مربوط به آن را نیز همراه خود دارند؛ برای جزئیات [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) را ببینید.

TL Agent یک پروژه‌ی مستقل است و محصول رسمی Runtime Upstream خود نیست.

این پروژه با هویت **TunnelLab** توسعه داده می‌شود.
