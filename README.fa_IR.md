<p align="center" dir="ltr"><a href="./README.md">English</a> · <a href="./README.fa_IR.md">فارسی</a></p>

<p align="center">
  <img src="./media/tl-studio-logo.svg" width="360" alt="TL Studio">
</p>

<p align="center" dir="rtl">
  <strong>یک محیط کدنویسی محلی با Agent هوش مصنوعی بومی.</strong>
</p>

<p align="center" dir="rtl">
  ویرایش، جست‌وجو، اجرا، Preview، گفتگو، Toolها و اتصال مدل‌ها؛ همه در یک Workspace محلی روی سیستم خودتان.
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/tl-studio"><img src="https://img.shields.io/npm/v/tl-studio?label=npm&color=CB3837" alt="npm version"></a>
  <a href="https://github.com/pouramin/TL-Studio/releases"><img src="https://img.shields.io/github/v/release/pouramin/TL-Studio?sort=semver&label=release" alt="Latest release"></a>
  <a href="https://github.com/pouramin/TL-Studio/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/pouramin/TL-Studio/ci.yml?branch=main&label=CI" alt="CI"></a>
  <a href="https://github.com/pouramin/TL-Studio/releases"><img src="https://img.shields.io/github/downloads/pouramin/TL-Studio/total?label=downloads" alt="Downloads"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/stable-v0.5.0-16a34a" alt="Stable v0.5.0">
  <img src="https://img.shields.io/badge/dev-v0.6.0--alpha.1-f59e0b" alt="Development v0.6.0-alpha.1">
</p>

---

<h2 dir="rtl" align="right"><bdi dir="ltr">TL Studio</bdi> چیست؟</h2>

<p dir="rtl" align="right"><strong><bdi dir="ltr">TL Studio</bdi></strong> یک محیط توسعهٔ محلی و مستقل است که حول یک <bdi dir="ltr">Agent</bdi> کدنویسی بومی ساخته شده است.</p>

<p dir="rtl" align="right">محیط مرورگری و <bdi dir="ltr">Backend</bdi> نوشته‌شده با <bdi dir="ltr">Go</bdi> به‌عنوان یک محصول واحد کار می‌کنند. چرخهٔ <bdi dir="ltr">Agent</bdi>، <bdi dir="ltr">Session</bdi>ها، <bdi dir="ltr">Tool</bdi>ها، <bdi dir="ltr">Permission</bdi>ها، <bdi dir="ltr">Provider</bdi>ها، <bdi dir="ltr">Credential</bdi>ها، <bdi dir="ltr">Process</bdi>های <bdi dir="ltr">Terminal</bdi>، فایل‌های پروژه، <bdi dir="ltr">Preview</bdi>، <bdi dir="ltr">Plugins/MCP</bdi> و <bdi dir="ltr">Event</bdi>های معنایی همگی در اختیار خود <bdi dir="ltr">TL Studio</bdi> هستند.</p>

<p dir="rtl" align="right"><bdi dir="ltr">Session</bdi>های کدنویسی از طریق <bdi dir="ltr">Native Agent</bdi> و <bdi dir="ltr">Tool Executor</bdi> خود <bdi dir="ltr">TL Studio</bdi> اجرا می‌شوند.</p>

<p dir="rtl" align="right">پروژه روی سیستم کاربر باقی می‌ماند و ترافیک مدل مستقیماً به <bdi dir="ltr">Provider</bdi> انتخاب‌شده ارسال می‌شود.</p>

<h2 dir="rtl" align="right">شروع سریع</h2>

<h3 dir="rtl" align="right">اجرا با <bdi dir="ltr">npm</bdi></h3>

<p dir="rtl" align="right">اگر <bdi dir="ltr">Node.js</bdi> و <bdi dir="ltr">npm</bdi> نصب هستند، داخل پوشهٔ پروژه این دستور را اجرا کنید:</p>

```bash
npx --yes tl-studio
```

<p dir="rtl" align="right">پکیج <bdi dir="ltr">npm Launcher</bdi> سبک نسخهٔ پایدار متناظر در <bdi dir="ltr">GitHub Releases</bdi> است. پلتفرم را تشخیص می‌دهد، <bdi dir="ltr">Binary</bdi> رسمی را دانلود می‌کند، <bdi dir="ltr">SHA-256</bdi> را بررسی می‌کند، فایل را در <bdi dir="ltr">Cache</bdi> محلی نگه می‌دارد و پوشهٔ فعلی را به‌عنوان پروژه باز می‌کند.</p>

<p dir="rtl" align="right">برای اجرا بدون باز شدن خودکار <bdi dir="ltr">Browser:</bdi></p>

```bash
npx --yes tl-studio --no-browser
```

<h3 dir="rtl" align="right">نسخهٔ <bdi dir="ltr">Portable</bdi></h3>

<p dir="rtl" align="right">نسخهٔ پایدار را از <strong>undefined</strong> دانلود و <bdi dir="ltr">Extract</bdi> کنید:</p>

```text
Windows   tl-studio.exe
Linux     ./tl-studio
macOS     ./tl-studio
```

<p dir="rtl" align="right">برای باز کردن یک پروژهٔ مشخص:</p>

```bash
tl-studio --project /path/to/project
```

<p dir="rtl" align="right">در <bdi dir="ltr">Windows:</bdi></p>

```powershell
.\tl-studio.exe --project C:\path\to\project
```

<h2 dir="rtl" align="right">قابلیت‌های اصلی</h2>

<table dir="rtl">
  <thead>
    <tr>
      <th align="right"></th>
      <th align="right">قابلیت</th>
      <th align="right">توضیح</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td align="right">🧠</td>
      <td align="right"><strong><bdi dir="ltr">Native Agent</bdi></strong></td>
      <td align="right">چرخهٔ کامل مدل → ابزار → مدل، لغو اجرا، <bdi dir="ltr">Loop Guard</bdi>، <bdi dir="ltr">Persistence</bdi> و پاسخ نهایی در اختیار <bdi dir="ltr">TL Studio</bdi> است.</td>
    </tr>
    <tr>
      <td align="right">🗂️</td>
      <td align="right"><strong><bdi dir="ltr">Workspace</bdi></strong></td>
      <td align="right"><bdi dir="ltr">File Explorer</bdi>، <bdi dir="ltr">Monaco Editor</bdi>، <bdi dir="ltr">Tab</bdi>ها، <bdi dir="ltr">Project Search</bdi>، عملیات فایل و هماهنگی با تغییرات بیرونی.</td>
    </tr>
    <tr>
      <td align="right">💻</td>
      <td align="right"><strong><bdi dir="ltr">Terminal</bdi></strong></td>
      <td align="right">اجرای دستورها در محدودهٔ پروژه، تاریخچهٔ خروجی، <bdi dir="ltr">Cancellation</bdi> و پایان <bdi dir="ltr">Process Tree</bdi> در سیستم‌های پشتیبانی‌شده.</td>
    </tr>
    <tr>
      <td align="right">👁️</td>
      <td align="right"><strong><bdi dir="ltr">Live Preview</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Preview</bdi> برای <bdi dir="ltr">HTML</bdi>، <bdi dir="ltr">Markdown</bdi>، تصویر، <bdi dir="ltr">PDF</bdi>، ویدیو، صدا، <bdi dir="ltr">SVG</bdi> و متن روی <bdi dir="ltr">Origin</bdi> محلی جداگانه.</td>
    </tr>
    <tr>
      <td align="right">🔐</td>
      <td align="right"><strong><bdi dir="ltr">Permissions</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Tool</bdi>های حساس می‌توانند برای <bdi dir="ltr">Approval</bdi> متوقف شوند و <bdi dir="ltr">Allow Once</bdi>، <bdi dir="ltr">Reject</bdi> یا <bdi dir="ltr">Rule</bdi>های <bdi dir="ltr">Project-scoped</bdi> داشته باشند.</td>
    </tr>
    <tr>
      <td align="right">💬</td>
      <td align="right"><strong><bdi dir="ltr">Interactive Questions</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Agent</bdi> می‌تواند سؤال ساختاریافته بپرسد، پاسخ گزینه‌ای یا متن دلخواه بگیرد و همان <bdi dir="ltr">Run</bdi> را ادامه دهد.</td>
    </tr>
    <tr>
      <td align="right">🔌</td>
      <td align="right"><strong><bdi dir="ltr">Providers</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Client</bdi>های مستقیم برای <bdi dir="ltr">OpenAI-compatible</bdi>، <bdi dir="ltr">OpenAI Responses</bdi>، <bdi dir="ltr">Anthropic Messages</bdi> و <bdi dir="ltr">Google Gemini.</bdi></td>
    </tr>
    <tr>
      <td align="right">👤</td>
      <td align="right"><strong><bdi dir="ltr">Provider Accounts</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Provider</bdi>های پشتیبانی‌شده می‌توانند از <bdi dir="ltr">Account Login</bdi> استفاده کنند بدون اینکه <bdi dir="ltr">Credential</bdi> وارد <bdi dir="ltr">Browser Code</bdi> شود.</td>
    </tr>
    <tr>
      <td align="right">🧩</td>
      <td align="right"><strong><bdi dir="ltr">Plugins</bdi> / <bdi dir="ltr">MCP</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Tool</bdi>های خارجی وارد همان <bdi dir="ltr">Tool Registry</bdi> و <bdi dir="ltr">Permission Boundary</bdi> می‌شوند.</td>
    </tr>
    <tr>
      <td align="right">📚</td>
      <td align="right"><strong><bdi dir="ltr">Sessions</bdi></strong></td>
      <td align="right">ساخت، <bdi dir="ltr">Rename</bdi>، <bdi dir="ltr">Resume</bdi>، <bdi dir="ltr">Delete</bdi>، <bdi dir="ltr">Abort</bdi>، <bdi dir="ltr">Persistence</bdi> و مشاهدهٔ تغییرات هر <bdi dir="ltr">Session.</bdi></td>
    </tr>
    <tr>
      <td align="right">📊</td>
      <td align="right"><strong><bdi dir="ltr">Usage</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Activity</bdi> و <bdi dir="ltr">Usage</bdi> هر <bdi dir="ltr">Session</bdi> بخشی از مدل معنایی خود <bdi dir="ltr">TL Studio</bdi> هستند.</td>
    </tr>
    <tr>
      <td align="right">🏠</td>
      <td align="right"><strong><bdi dir="ltr">Local-first</bdi></strong></td>
      <td align="right"><bdi dir="ltr">Cloud Backend</bdi>، <bdi dir="ltr">Database</bdi>، <bdi dir="ltr">Hosted Proxy</bdi> یا <bdi dir="ltr">Telemetry Service</bdi> متعلق به <bdi dir="ltr">TL Studio</bdi> لازم نیست.</td>
    </tr>
  </tbody>
</table>

<h2 dir="rtl" align="right"><bdi dir="ltr">Provider</bdi>ها</h2>

<p dir="rtl" align="right"><bdi dir="ltr">Credential</bdi> دستی <bdi dir="ltr">API</bdi> و <bdi dir="ltr">Credential</bdi> مربوط به <bdi dir="ltr">Account Login</bdi> در <bdi dir="ltr">Slot</bdi>های جداگانهٔ <bdi dir="ltr">Credential Vault</bdi> نگهداری می‌شوند. اتصال حساب می‌تواند در زمان فعال بودن اولویت داشته باشد، اما <bdi dir="ltr">Sign out</bdi> کردن حساب <bdi dir="ltr">API Key</bdi> دستی موجود را حذف نمی‌کند.</p>

<p dir="rtl" align="right"><bdi dir="ltr">Secret</bdi>ها در این محل‌ها ذخیره نمی‌شوند:</p>

```text
providers.json
Browser localStorage
sessionStorage
frontend source
normal local API responses
```

<h3 dir="rtl" align="right"><bdi dir="ltr">Protocol</bdi>های <bdi dir="ltr">Native</bdi> مدل</h3>

<table dir="rtl">
  <thead>
    <tr>
      <th align="right"><bdi dir="ltr">Protocol</bdi></th>
      <th align="right">وضعیت</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td align="right"><bdi dir="ltr">OpenAI-compatible Chat Completions</bdi></td>
      <td align="right">✅ پشتیبانی می‌شود</td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">OpenAI Responses</bdi></td>
      <td align="right">✅ پشتیبانی می‌شود</td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">Anthropic Messages</bdi></td>
      <td align="right">✅ پشتیبانی می‌شود</td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">Google Gemini generateContent</bdi></td>
      <td align="right">✅ پشتیبانی می‌شود</td>
    </tr>
  </tbody>
</table>

<p dir="rtl" align="right">راه‌اندازی <bdi dir="ltr">Provider</bdi> سفارشی <bdi dir="ltr">Discovery-first</bdi> است. در حالت عادی فقط <bdi dir="ltr">API Address</bdi>، <bdi dir="ltr">API Type</bdi> و <bdi dir="ltr">API Key</bdi> وارد می‌شوند و <bdi dir="ltr">TL Studio</bdi> مدل‌های قابل استفاده را به‌صورت خودکار کشف می‌کند.</p>

<p dir="rtl" align="right">ورود دستی <bdi dir="ltr">Model</bdi> فقط به‌عنوان <bdi dir="ltr">Fallback</bdi> صریح برای مدل‌های خصوصی یا فهرست‌نشده باقی می‌ماند.</p>

<h3 dir="rtl" align="right"><bdi dir="ltr">Provider</bdi>های حسابی</h3>

<table dir="rtl">
  <thead>
    <tr>
      <th align="right"><bdi dir="ltr">Provider</bdi></th>
      <th align="right"><bdi dir="ltr">Account Login</bdi></th>
      <th align="right">توضیح</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td align="right"><strong><bdi dir="ltr">OpenRouter</bdi></strong></td>
      <td align="right">✅ فعال</td>
      <td align="right"><bdi dir="ltr">OAuth</bdi> + <bdi dir="ltr">PKCE</bdi> رسمی، <bdi dir="ltr">Credential</bdi> مبتنی بر حساب و <bdi dir="ltr">Model Discovery.</bdi></td>
    </tr>
    <tr>
      <td align="right"><strong><bdi dir="ltr">Hugging Face</bdi></strong></td>
      <td align="right">✅ فعال</td>
      <td align="right"><bdi dir="ltr">Public-client OAuth</bdi> + <bdi dir="ltr">PKCE</bdi>، <bdi dir="ltr">Refresh</bdi> و <bdi dir="ltr">Model Discovery</bdi> برای <bdi dir="ltr">Inference Providers.</bdi></td>
    </tr>
    <tr>
      <td align="right"><strong><bdi dir="ltr">Google</bdi> / <bdi dir="ltr">Gemini</bdi></strong></td>
      <td align="right">✅ فعال</td>
      <td align="right"><bdi dir="ltr">Installed-app OAuth</bdi> + <bdi dir="ltr">PKCE</bdi>، <bdi dir="ltr">Refresh/Revocation</bdi>، <bdi dir="ltr">Transport</bdi> بومی <bdi dir="ltr">Gemini</bdi> و پشتیبانی از <bdi dir="ltr">Google Cloud quota project.</bdi></td>
    </tr>
    <tr>
      <td align="right"><strong><bdi dir="ltr">ChatGPT</bdi> / <bdi dir="ltr">Codex</bdi></strong></td>
      <td align="right">⏳ <bdi dir="ltr">Deferred</bdi></td>
      <td align="right">همچنان یکی از هدف‌های اصلی 0.6 است، اما سطح رسمی فعلی <bdi dir="ltr">Transport</bdi> خام مدل موردنیاز برای حفظ مرز <bdi dir="ltr">Native Agent</bdi> را ارائه نمی‌کند.</td>
    </tr>
    <tr>
      <td align="right"><strong><bdi dir="ltr">Claude account</bdi></strong></td>
      <td align="right">⏳ <bdi dir="ltr">Deferred</bdi></td>
      <td align="right">پشتیبانی <bdi dir="ltr">Anthropic</bdi> با <bdi dir="ltr">API Key</bdi> بومی است؛ <bdi dir="ltr">Login</bdi> حساب مصرف‌کننده منتظر یک قرارداد عمومی و مستند برای <bdi dir="ltr">Third-party authorization</bdi> می‌ماند.</td>
    </tr>
    <tr>
      <td align="right"><strong><bdi dir="ltr">GitHub Copilot</bdi></strong></td>
      <td align="right">⏳ <bdi dir="ltr">Deferred</bdi></td>
      <td align="right">احراز هویت رسمی وجود دارد، اما مسیر مستند <bdi dir="ltr">Model Access</bdi> همچنان به <bdi dir="ltr">Copilot SDK/runtime</bdi> وابسته است.</td>
    </tr>
  </tbody>
</table>

<p dir="rtl" align="right"><bdi dir="ltr">Provider</bdi>های <bdi dir="ltr">Deferred</bdi> عمداً در <bdi dir="ltr">Settings</bdi> دیده می‌شوند. آن‌ها مرز معماری مشخص هستند، نه قابلیت‌های فراموش‌شده.</p>

<h3 dir="rtl" align="right">تنظیمات <bdi dir="ltr">Gemini</bdi> در نسخهٔ <bdi dir="ltr">Alpha</bdi></h3>

<p dir="rtl" align="right">در خط توسعهٔ 0.6 این مقادیر غیرمحرمانه مستقیماً از <bdi dir="ltr">Provider Settings</bdi> قابل تنظیم هستند:</p>

```text
Google Cloud Project ID
Desktop OAuth Client ID
```

<p dir="rtl" align="right"><bdi dir="ltr">Access Token</bdi> و <bdi dir="ltr">Refresh Token</bdi> فقط داخل <bdi dir="ltr">Credential Vault</bdi> خود <bdi dir="ltr">TL Studio</bdi> باقی می‌مانند.</p>

<p dir="rtl" align="right">برای <bdi dir="ltr">Login</bdi> دسکتاپ <bdi dir="ltr">Gemini</bdi> یک <bdi dir="ltr">Listener</bdi> موقت روی <bdi dir="ltr">Loopback</bdi> ساخته می‌شود:</p>

```text
127.0.0.1:<random-port>
```

<p dir="rtl" align="right">این <bdi dir="ltr">Listener</bdi> مقدار <bdi dir="ltr">OAuth state</bdi> را بررسی می‌کند، <bdi dir="ltr">Flow</bdi> مبتنی بر <bdi dir="ltr">PKCE</bdi> را کامل می‌کند و پس از موفقیت، لغو یا <bdi dir="ltr">Expiry</bdi> بسته می‌شود.</p>

<h2 dir="rtl" align="right">معماری</h2>

<p dir="rtl" align="right"><bdi dir="ltr">TL Studio</bdi> یک مرز محصول بومی و یکپارچه دارد:</p>

```mermaid
flowchart TD
    UI["Browser workspace"] --> API["TL Studio local API"]
    API --> SESSION["Sessions / permissions / questions / events"]
    SESSION --> AGENT["Native Agent"]
    AGENT --> PROVIDER["Selected model provider"]
    AGENT --> TOOLS["TL Studio Tool Executor"]

    TOOLS --> FILES["Project files & search"]
    TOOLS --> TERM["Terminal / processes"]
    TOOLS --> MCP["Plugins / MCP"]

    PROVIDER --> AGENT
    TOOLS --> AGENT
    AGENT --> SESSION
    SESSION --> UI
```

<p dir="rtl" align="right">بخش‌های زیر مستقیماً در اختیار <bdi dir="ltr">TL Studio</bdi> هستند:</p>

<ul dir="rtl">
  <li><bdi dir="ltr">Workspace</bdi> و <bdi dir="ltr">Monaco Editor</bdi></li>
  <li>فایل‌ها و <bdi dir="ltr">Project Search</bdi></li>
  <li><bdi dir="ltr">Terminal</bdi> و <bdi dir="ltr">Process execution</bdi></li>
  <li><bdi dir="ltr">Preview</bdi></li>
  <li><bdi dir="ltr">Provider Registry</bdi> و <bdi dir="ltr">Model Discovery</bdi></li>
  <li><bdi dir="ltr">Credential Vault</bdi></li>
  <li>چرخهٔ <bdi dir="ltr">Provider Account</bdi></li>
  <li><bdi dir="ltr">Native Agent</bdi></li>
  <li><bdi dir="ltr">Session</bdi>ها و <bdi dir="ltr">Persistence</bdi></li>
  <li><bdi dir="ltr">Question</bdi>ها و <bdi dir="ltr">Permission</bdi>ها</li>
  <li><bdi dir="ltr">Event</bdi>های معنایی</li>
  <li><bdi dir="ltr">Tool Registry</bdi> و <bdi dir="ltr">Tool Executor</bdi></li>
  <li><bdi dir="ltr">Plugins/MCP</bdi></li>
</ul>

<p dir="rtl" align="right"><bdi dir="ltr">Protocol</bdi> پشتیبانی‌نشده با خطای صریح <bdi dir="ltr">Unsupported Capability</bdi> متوقف می‌شود.</p>

<p dir="rtl" align="right">جزئیات کامل‌تر:</p>

<p dir="rtl" align="right"><a href="./docs/ARCHITECTURE.md"><bdi dir="ltr">docs/ARCHITECTURE.md</bdi></a></p>

<h2 dir="rtl" align="right">مدل امنیتی</h2>

<p dir="rtl" align="right"><bdi dir="ltr">Control Surface</bdi> برنامه برای استفادهٔ محلی طراحی شده است:</p>

<ul dir="rtl">
  <li><bdi dir="ltr">Control UI</bdi> فقط روی <bdi dir="ltr">Loopback Bind</bdi> می‌شود.</li>
  <li><bdi dir="ltr">Host</bdi>های غیرمحلی رد می‌شوند.</li>
  <li><bdi dir="ltr">Origin</bdi> مرورگر باید با <bdi dir="ltr">Control Origin</bdi> محلی یکسان باشد.</li>
  <li>عملیات <bdi dir="ltr">Filesystem</bdi> مرز پروژه را <bdi dir="ltr">enforce</bdi> می‌کنند.</li>
  <li><bdi dir="ltr">Traversal</bdi> و <bdi dir="ltr">Symlink Escape</bdi> رد می‌شوند.</li>
  <li><bdi dir="ltr">Preview</bdi> از <bdi dir="ltr">Control Origin</bdi> جداست.</li>
  <li><bdi dir="ltr">Secret</bdi>های <bdi dir="ltr">Provider</bdi> وارد <bdi dir="ltr">Browser Code</bdi> نمی‌شوند.</li>
  <li><bdi dir="ltr">Tool</bdi>های حساس پشت <bdi dir="ltr">Permission</bdi> قرار دارند.</li>
</ul>

<p dir="rtl" align="right"><bdi dir="ltr">Provider</bdi>های خارجی، <bdi dir="ltr">Repository</bdi>ها، <bdi dir="ltr">Prompt</bdi>ها، <bdi dir="ltr">MCP Server</bdi>ها و <bdi dir="ltr">Plugin Process</bdi>ها مرزهای اعتماد مستقل هستند.</p>

<p dir="rtl" align="right"><bdi dir="ltr">Control Port</bdi> برنامه را از طریق <bdi dir="ltr">Public Proxy</bdi> در معرض اینترنت قرار ندهید.</p>

<p dir="rtl" align="right">جزئیات بیشتر:</p>

<p dir="rtl" align="right"><a href="./SECURITY.md"><bdi dir="ltr">SECURITY.md</bdi></a></p>

<h2 dir="rtl" align="right">نسخه‌های قابل اجرا</h2>

<table dir="rtl">
  <thead>
    <tr>
      <th align="right">پلتفرم</th>
      <th align="right">معماری</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td align="right"><bdi dir="ltr">Windows</bdi></td>
      <td align="right"><bdi dir="ltr">x64</bdi></td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">Linux</bdi></td>
      <td align="right"><bdi dir="ltr">x64</bdi>, <bdi dir="ltr">ARM64</bdi></td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">macOS</bdi></td>
      <td align="right"><bdi dir="ltr">Intel x64</bdi>, <bdi dir="ltr">Apple Silicon ARM64</bdi></td>
    </tr>
  </tbody>
</table>

<p dir="rtl" align="right">نسخه‌های پایدار همراه با <bdi dir="ltr">SHA-256</bdi> از طریق <strong>undefined</strong> منتشر می‌شوند.</p>

<h2 dir="rtl" align="right"><bdi dir="ltr">Build</bdi> از <bdi dir="ltr">Source</bdi></h2>

<h3 dir="rtl" align="right">نیازمندی‌ها</h3>

```text
Go 1.23+
Node.js 18+
npm
```

<p dir="rtl" align="right">نصب وابستگی‌های توسعهٔ <bdi dir="ltr">Browser:</bdi></p>

```bash
npm install --ignore-scripts --no-audit --no-fund
```

<p dir="rtl" align="right"><bdi dir="ltr">Type-check</bdi> و <bdi dir="ltr">Build</bdi> رابط <bdi dir="ltr">Browser:</bdi></p>

```bash
npm run check:web
npm run build:web
```

<p dir="rtl" align="right">اجرای <bdi dir="ltr">Test</bdi>ها و <bdi dir="ltr">Vet:</bdi></p>

```bash
go test ./...
go vet ./...
```

<p dir="rtl" align="right">اجرای <bdi dir="ltr">TL Studio:</bdi></p>

```bash
go run ./cmd/launcher --project /path/to/project
```

<p dir="rtl" align="right">ساخت <bdi dir="ltr">Binary</bdi> محلی:</p>

```bash
go build -o tl-studio ./cmd/launcher
```

<p dir="rtl" align="right">در <bdi dir="ltr">Windows:</bdi></p>

```powershell
go build -o tl-studio.exe ./cmd/launcher
```

<p dir="rtl" align="right">اجرای عادی برنامه <bdi dir="ltr">Native</bdi> است و <bdi dir="ltr">Flag</bdi> جداگانه یا <bdi dir="ltr">Runtime</bdi> خارجی دیگری لازم ندارد.</p>

<h2 dir="rtl" align="right">اعتبارسنجی</h2>

<p dir="rtl" align="right"><bdi dir="ltr">Validation</bdi> خودکار <bdi dir="ltr">Repository</bdi> فقط <bdi dir="ltr">Compilation</bdi> را بررسی نمی‌کند و مسیرهای اصلی محصول را هم پوشش می‌دهد.</p>

<p dir="rtl" align="right">بررسی‌ها، بسته به نوع تغییر، شامل این موارد هستند:</p>

<ul dir="rtl">
  <li><bdi dir="ltr">Type-check</bdi> سخت‌گیرانهٔ <bdi dir="ltr">TypeScript</bdi> برای <bdi dir="ltr">Browser</bdi></li>
  <li><bdi dir="ltr">Build</bdi> نهایی <bdi dir="ltr">Browser</bdi></li>
  <li><code dir="ltr">go test ./...</code></li>
  <li><code dir="ltr">go vet ./...</code></li>
  <li><bdi dir="ltr">Cross-compile</bdi> برای پلتفرم‌های پشتیبانی‌شده</li>
  <li><bdi dir="ltr">Native Product Contract</bdi></li>
  <li><bdi dir="ltr">Custom Provider Contract</bdi></li>
  <li><bdi dir="ltr">Browser Smoke</bdi></li>
  <li><bdi dir="ltr">Agent E2E</bdi></li>
  <li><bdi dir="ltr">Regression Test</bdi>های <bdi dir="ltr">Provider</bdi></li>
  <li><bdi dir="ltr">Windows x64 Review Package</bdi></li>
  <li>بررسی <bdi dir="ltr">Release</bdi> و <bdi dir="ltr">Review Package</bdi></li>
</ul>

<p dir="rtl" align="right">در <bdi dir="ltr">CI</bdi> از <bdi dir="ltr">Credential</bdi> واقعی استفاده نمی‌شود و <bdi dir="ltr">Provider</bdi>ها با <bdi dir="ltr">Endpoint</bdi>های <bdi dir="ltr">Mock</bdi> و <bdi dir="ltr">Contract Test</bdi> بررسی می‌شوند.</p>

<h2 dir="rtl" align="right">شاخه‌ها و کانال انتشار</h2>

<table dir="rtl">
  <thead>
    <tr>
      <th align="right">شاخه / کانال</th>
      <th align="right">کاربرد</th>
      <th align="right">نسخهٔ فعلی</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td align="right"><code dir="ltr">main</code></td>
      <td align="right">نسخهٔ پایدار</td>
      <td align="right"><strong><bdi dir="ltr">v0.5.0</bdi></strong></td>
    </tr>
    <tr>
      <td align="right"><code dir="ltr">dev</code></td>
      <td align="right">توسعهٔ فعال</td>
      <td align="right"><strong><bdi dir="ltr">v0.6.0-alpha.1</bdi></strong></td>
    </tr>
    <tr>
      <td align="right"><bdi dir="ltr">Feature Branch</bdi></td>
      <td align="right">تغییر متمرکز بر پایهٔ <bdi dir="ltr">dev</bdi></td>
      <td align="right">کوتاه‌عمر</td>
    </tr>
  </tbody>
</table>

<p dir="rtl" align="right">نسخهٔ پایدار تنها زمانی به <bdi dir="ltr">main</bdi> ارتقا پیدا می‌کند که <bdi dir="ltr">Milestone</bdi> مربوط <bdi dir="ltr">Validation</bdi> خودکار و <bdi dir="ltr">Review</bdi> عملی را پشت سر گذاشته باشد.</p>

<p dir="rtl" align="right"><bdi dir="ltr">Launcher</bdi> پایدار <bdi dir="ltr">npm</bdi> از <bdi dir="ltr">Release</bdi> پایدار پیروی می‌کند و <bdi dir="ltr">Development Build</bdi>ها به‌صورت جداگانه از <bdi dir="ltr">dev</bdi> ساخته می‌شوند.</p>

<h2 dir="rtl" align="right">اصول پروژه</h2>

<ol dir="rtl">
  <li><strong><bdi dir="ltr">Local-first</bdi></strong> — <bdi dir="ltr">Workspace</bdi> و پروژه روی سیستم خود کاربر باقی می‌مانند.</li>
  <li><strong><bdi dir="ltr">Native execution</bdi></strong> — چرخهٔ <bdi dir="ltr">Agent</bdi> و <bdi dir="ltr">Tool</bdi> متعلق به خود <bdi dir="ltr">TL Studio</bdi> است.</li>
  <li><strong>مرز اعتماد شفاف</strong> — <bdi dir="ltr">Credential</bdi>ها، <bdi dir="ltr">Permission</bdi>ها، <bdi dir="ltr">Provider</bdi>ها و <bdi dir="ltr">Tool</bdi>های خارجی مرز مشخص دارند.</li>
  <li><strong>مرز قابلیت شفاف</strong> — قابلیت پشتیبانی‌نشده به‌صورت واضح <bdi dir="ltr">Fail</bdi> می‌شود و با قابلیت دیگری جایگزین نمی‌شود.</li>
  <li><strong><bdi dir="ltr">Discovery-first Provider setup</bdi></strong> — راه‌اندازی <bdi dir="ltr">Provider</bdi>های معمول باید تا حد ممکن ساده باشد.</li>
  <li><strong><bdi dir="ltr">Portable by default</bdi></strong> — <bdi dir="ltr">Binary</bdi> پایدار برای <bdi dir="ltr">Windows</bdi>، <bdi dir="ltr">Linux</bdi> و <bdi dir="ltr">macOS</bdi> منتشر می‌شود.</li>
  <li><strong>بدون زیرساخت اختصاصی پروژه</strong> — <bdi dir="ltr">Backend</bdi> میزبانی‌شده، <bdi dir="ltr">Database</bdi>، <bdi dir="ltr">Telemetry Service</bdi> یا <bdi dir="ltr">Model Proxy</bdi> برای استفاده از <bdi dir="ltr">TL Studio</bdi> لازم نیست.</li>
</ol>

<h2 dir="rtl" align="right">ساختار <bdi dir="ltr">Repository</bdi></h2>

```text
cmd/launcher/          Go application, local APIs, Agent, providers, tools
cmd/launcher/ui/       Browser TypeScript source
docs/                  Architecture and design documentation
media/                 Project branding
packaging/             Release/package support
scripts/               Browser build and launcher tooling
third_party/           Required third-party notices/licenses
```

<p dir="rtl" align="right">اسناد مهم:</p>

<ul dir="rtl">
  <li><a href="./docs/ARCHITECTURE.md">معماری</a></li>
  <li><a href="./SECURITY.md">امنیت</a></li>
  <li><a href="./README.md"><bdi dir="ltr">README</bdi> انگلیسی</a></li>
  <li><a href="./THIRD_PARTY_NOTICES.md"><bdi dir="ltr">Third-party notices</bdi></a></li>
</ul>

<h2 dir="rtl" align="right"><bdi dir="ltr">License</bdi></h2>

<p dir="rtl" align="right"><bdi dir="ltr">TL Studio</bdi> تحت <strong><bdi dir="ltr">MIT License</bdi></strong> منتشر می‌شود.</p>

<p dir="rtl" align="right">فایل‌های مربوط:</p>

<p dir="rtl" align="right"><a href="./LICENSE"><bdi dir="ltr">LICENSE</bdi></a></p>

<p dir="rtl" align="right"><a href="./THIRD_PARTY_NOTICES.md"><bdi dir="ltr">THIRD_PARTY_NOTICES.md</bdi></a></p>

---

<p dir="rtl" align="right"><<bdi dir="ltr">p align</bdi>="<bdi dir="ltr">center</bdi>"> ساخته‌شده تحت هویت <<bdi dir="ltr">strong</bdi>><bdi dir="ltr">TunnelLab</bdi></<bdi dir="ltr">strong</bdi>>. </<bdi dir="ltr">p</bdi>></p>
