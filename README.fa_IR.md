# TL Studio

<p align="center">
  <img src="./media/tl-studio-logo.svg" width="360" alt="TL Studio">
</p>

TL Studio یک IDE مرورگری و Workspace محلی برای Coding Agent است. Workspace، Editor، Search، Terminal، Preview، Sessionها، Permissionها، Questionها، Providerها، Plugin/MCP و Native Agent همگی پشت قراردادهای محلی خود TL Studio اجرا می‌شوند.

نسخه‌ی پایدار فعلی: **0.5.0**

خط توسعه‌ی فعلی: **0.6.0-alpha.1**

شاخه‌ی main روی v0.5.0 پایدار می‌ماند و توسعه‌ی جدید روی dev ادامه پیدا می‌کند.

## معماری Native

اجرای عادی TL Studio دیگر به Sidecar یا Compatibility Runtime وابسته نیست.

- **Native Agent** — حلقه‌ی Model/Tool/Model، Cancellation، Loop Guard، Persistence معنایی و Live Event متعلق به TL Studio است.
- **Native Sessions** — Create، Rename، Delete، Run، Abort، Status، Message، Changes و Persistence کاملاً در اختیار TL Studio است.
- **Native Interactive Questions** — Agent می‌تواند با interaction.question متوقف شود، سؤال را به Browser بفرستد، پاسخ Multiple Choice یا Custom Text بگیرد و ادامه دهد.
- **Native Permissions** — Pending Request، Allow Once، Reject، Ruleهای Project-scoped و Enforcement در اختیار TL Studio است.
- **Native Events** — مسیر /local/events منبع اصلی Semantic SSE است.
- **Native Providers** — Provider Registry، Catalog، Discovery و Credential Vault متعلق به TL Studio است و Model Call مستقیماً به Provider انتخاب‌شده ارسال می‌شود.
- **Provider Accounts** — مسیر /local/provider-accounts چرخه‌ی عمومی ورود حساب برای Integrationهای مستند Provider را مدیریت می‌کند و OpenRouter، Hugging Face و Google/Gemini روی خط توسعه‌ی 0.6 پیاده‌سازی شده‌اند.
- **Native Tools** — Files، Search، Terminal/Process، Workspace reconciliation و ابزارهای Plugin/MCP از TL Studio Tool Executor عبور می‌کنند.

در محصول دیگر Kilo binary، Kilo subprocess، Local Kilo server، Reverse Proxy، Session Adapter، Permission fallback، Question Adapter، Event stream، Provider sync، Model fallback یا Kilo داخل Release package وجود ندارد.

اگر کاربر بخواهد، Kilo Gateway فقط می‌تواند مانند هر Provider خارجی دیگر از طریق Endpoint عمومی HTTPS و API Key مستندشده تنظیم شود. این حالت هیچ وابستگی محلی به Kilo ایجاد نمی‌کند.

## Workspace

Browser Workspace شامل این بخش‌هاست:

- File Explorer با Create، Rename، Delete، Save، Refresh و Optimistic Concurrency؛
- Monaco Editor که به‌صورت Local bundle شده و CDN لازم ندارد؛
- Project Search؛
- Terminal/Process پروژه با Stop؛
- Live Preview روی Loopback origin جدا؛
- Agent conversation، Sessionها، Usage/Activity، Changes، Permission و Interactive Question؛
- Provider Settings و Credential Vault؛
- Plugin/MCP با Configuration و Tool Discovery؛
- JEV/OpenRouter از مسیر Native Provider.

## Providerها

TL Studio در حال حاضر Direct Model Client برای این Protocolها دارد:

- OpenAI-compatible Chat Completions
- OpenAI Responses
- Anthropic Messages
- Google Gemini generateContent

Provider definition در Local State خود TL Studio ذخیره می‌شود. API Key و Credential حساب جداگانه داخل Credential Vault نگهداری می‌شوند و وارد providers.json یا Browser storage نمی‌شوند. Credential دستی API و اتصال حساب Slotهای جدا دارند و اتصال حساب، کلید دستی کاربر را overwrite نمی‌کند.

در افزودن Provider سفارشی، TL Studio می‌تواند فهرست مدل‌ها را به‌صورت خودکار از API سازگار کشف کند. ورود دستی Model ID فقط به‌عنوان fallback برای مدل خصوصی یا فهرست‌نشده باقی می‌ماند.

زیرساخت Account Provider در خط توسعه‌ی 0.6 شامل وضعیت معنایی Login، لغو، Refresh، Logout و Model Discovery است. OpenRouter از OAuth + PKCE مستند خودش استفاده می‌کند، Hugging Face با Public OAuth Client و PKCE متصل می‌شود و Google/Gemini نیز از Installed App OAuth، PKCE، Refresh و Revocation رسمی استفاده می‌کند. Credential هیچ‌کدام در اختیار Browser قرار نمی‌گیرد.

اتصال حساب Google به یک Desktop OAuth Client ثبت‌شده برای TL Studio و یک Google Cloud Project با Generative Language API فعال نیاز دارد. اشتراک مصرف‌کننده‌ی Gemini به‌عنوان Gemini API entitlement در نظر گرفته نمی‌شود.

اتصال حساب ChatGPT/Codex، Claude و GitHub Copilot وقتی قرارداد رسمی فعلی با مرز Native execution پروژه سازگار نباشد، در Provider Settings به‌صورت شفاف غیرفعال نمایش داده می‌شود. این موارد Hidden fallback نیستند و تا زمان حل مرز معماری، به Runtime credential resolver ثبت نمی‌شوند.

اگر Model یا Protocol توسط Native Client پشتیبانی نشود، خطای Unsupported Capability به‌صورت شفاف برمی‌گردد. هیچ Hidden fallback به Runtime دیگری وجود ندارد.

JEV هم از همان Native Agent، Tool Executor، Permission، Session و Event system استفاده می‌کند.

## Local Product API

Browser فقط از Semantic APIهای خود TL Studio استفاده می‌کند؛ از جمله:

- /local/status
- /local/health
- /local/path
- /local/agents
- /local/providers/*
- /local/provider-accounts*
- /local/sessions*
- /local/questions*
- /local/permissions*
- /local/events
- /local/plugins*
- /local/tools

معماری قدیمی /runtime/* Reverse Proxy دیگر بخشی از محصول نیست.

## Build از Source

نیازمندی‌ها:

- Go 1.23+
- Node.js 18+ برای Build و Check رابط Browser

ساخت و بررسی Browser:

    npm install --ignore-scripts --no-audit --no-fund
    npm run check:web
    npm run build:web

اجرای Testها:

    go test ./...
    go vet ./...

اجرای TL Studio:

    go run ./cmd/launcher --project /path/to/project

ساخت Binary:

    go build -o tl-studio ./cmd/launcher
    ./tl-studio --project /path/to/project

در Windows:

    go build -o tl-studio.exe ./cmd/launcher
    .\tl-studio.exe --project C:\path\to\project

اجرای عادی همان Native execution است. Flag جداگانه‌ای برای Native-only وجود ندارد و Runtime binary دیگری هم لازم نیست.

## Release Packaging

Package عادی شامل TL Studio executable، License/Noticeهای TL Studio و Pluginهای bundle‌شده‌ی خود TL Studio است. فایل kilo یا kilo.exe داخل Package وجود ندارد.

CI اگر یکی از این Binaryها وارد Review یا Release package شود Fail می‌شود. CI همچنین TL Studio standalone را به‌صورت عادی اجرا می‌کند، Native Product Contract و Browser smoke واقعی را تست می‌کند و Windows x64 Review ZIP می‌سازد.

## Security

TL Studio به‌صورت Local-first طراحی شده است. Control UI فقط روی Loopback bind می‌شود، Cross-origin request رد می‌شود، Filesystem API مرز Project را enforce می‌کند، Preview از Control Origin جداست، Credentialها در Vault خود TL Studio می‌مانند و Toolهای حساس Permission می‌خواهند.

External Provider، Repository، Prompt و MCP/Plugin process مرزهای اعتماد جداگانه‌اند.

جزئیات بیشتر در SECURITY.md است.

## Branchها

- main — Stable production
- dev — Active next-version development
- Feature branchها — کار milestone جدا از dev

جزئیات معماری در docs/ARCHITECTURE.md است.

## License

TL Studio تحت MIT منتشر می‌شود. برای نرم‌افزارهای شخص ثالثی که همچنان همراه TL Studio توزیع می‌شوند، LICENSE و THIRD_PARTY_NOTICES.md را ببینید.
