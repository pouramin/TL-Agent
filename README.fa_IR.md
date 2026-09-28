# TL Studio

<p align="center">
  <img src="./media/tl-studio-logo.svg" width="360" alt="TL Studio">
</p>

TL Studio یک IDE مرورگری و Workspace محلی برای Coding Agent است. Workspace، Editor، Search، Terminal، Preview، Sessionها، Permissionها، Questionها، Providerها، Plugin/MCP و Native Agent همگی پشت قراردادهای محلی خود TL Studio اجرا می‌شوند.

خط توسعه‌ی فعلی: **0.5.0-alpha.3**

شاخه‌ی main تا زمان Review و Merge این milestone روی خط Stable نسخه‌ی v0.4.0 باقی می‌ماند.

## معماری Native

اجرای عادی TL Studio دیگر به Sidecar یا Compatibility Runtime وابسته نیست.

- **Native Agent** — حلقه‌ی Model/Tool/Model، Cancellation، Loop Guard، Persistence معنایی و Live Event متعلق به TL Studio است.
- **Native Sessions** — Create، Rename، Delete، Run، Abort، Status، Message، Changes و Persistence کاملاً در اختیار TL Studio است.
- **Native Interactive Questions** — Agent می‌تواند با interaction.question متوقف شود، سؤال را به Browser بفرستد، پاسخ Multiple Choice یا Custom Text بگیرد و ادامه دهد.
- **Native Permissions** — Pending Request، Allow Once، Reject، Ruleهای Project-scoped و Enforcement در اختیار TL Studio است.
- **Native Events** — مسیر /local/events منبع اصلی Semantic SSE است.
- **Native Providers** — Provider Registry، Catalog، Discovery و Credential Vault متعلق به TL Studio است و Model Call مستقیماً به Provider انتخاب‌شده ارسال می‌شود.
- **Provider Accounts** — مسیر /local/provider-accounts یک Domain عمومی برای Integrationهای Account-based آینده باقی مانده است.
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

Provider definition در Local State خود TL Studio ذخیره می‌شود. API Key جداگانه داخل Credential Vault نگهداری می‌شود و وارد providers.json یا Browser storage نمی‌شود.

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
