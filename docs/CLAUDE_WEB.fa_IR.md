<p align="center" dir="ltr"><a href="./CLAUDE_WEB.md">English</a> · <a href="./CLAUDE_WEB.fa_IR.md">فارسی</a></p>

<h1 dir="rtl" align="right"><span dir="ltr">Claude Web (Free/Pro)</span></h1>

<p dir="rtl" align="right"><span dir="ltr">TL Studio</span> می‌تواند از نشست <span dir="ltr">Claude</span> که در پروفایل عادی <span dir="ltr">Chrome</span> وارد شده استفاده کند. این اتصال از طریق افزونهٔ <strong><span dir="ltr">TL Studio Claude Web Bridge</span></strong> انجام می‌شود.</p>

<h2 dir="rtl" align="right">پیش‌نیاز</h2>

<p dir="rtl" align="right">افزونه باید در همان پروفایل <span dir="ltr">Chrome</span> نصب باشد که داخل <span dir="ltr">claude.ai</span> وارد شده است.</p>

<p dir="rtl" align="right"><a href="https://chromewebstore.google.com/detail/cpellhbmfdhcgkblnmnppndmeiigmjcg">نصب افزونهٔ <span dir="ltr">TL Studio Claude Web Bridge</span> از <span dir="ltr">Chrome Web Store</span></a></p>

<p dir="rtl" align="right">صفحهٔ افزونه در فروشگاه به‌صورت <span dir="ltr">Unlisted</span> منتشر شده است و برای نسخهٔ اصلی نیازی به <span dir="ltr">Developer Mode</span> نیست.</p>

<h2 dir="rtl" align="right">راه‌اندازی</h2>

<ol dir="rtl">
  <li>در <span dir="ltr">TL Studio</span> وارد <span dir="ltr">Settings → Providers</span> شوید.</li>
  <li>در کارت <span dir="ltr">Claude</span> گزینهٔ <span dir="ltr">Web</span> را انتخاب کنید.</li>
  <li>اگر افزونه نصب نباشد، <span dir="ltr">TL Studio</span> صفحهٔ نصب در <span dir="ltr">Chrome Web Store</span> را به‌صورت خودکار باز می‌کند و همان فرایند اتصال را در حالت انتظار نگه می‌دارد.</li>
  <li>افزونه را در همان پروفایل مرورگر نصب کنید و پنجرهٔ اتصال <span dir="ltr">TL Studio</span> را باز نگه دارید.</li>
  <li><span dir="ltr">TL Studio</span> افزونهٔ تازه‌نصب‌شده را به‌صورت خودکار تشخیص می‌دهد، نشست <span dir="ltr">Claude</span> را بررسی می‌کند و بدون نیاز به کلیک دوباره روی <span dir="ltr">Web</span>، <span dir="ltr">Pairing</span> را ادامه می‌دهد.</li>
  <li>مطمئن شوید همان پروفایل داخل <span dir="ltr">claude.ai</span> وارد شده است.</li>
  <li>وقتی کارت وضعیت <span dir="ltr">Web connected</span> را نشان داد، مدل‌های <span dir="ltr">Claude Web</span> در انتخاب‌گر مدل قابل استفاده هستند.</li>
</ol>

<h2 dir="rtl" align="right">چرخهٔ <span dir="ltr">Pairing</span></h2>

<p dir="rtl" align="right">برای هر اتصال یک <span dir="ltr">Token</span> تصادفی ساخته می‌شود و افزونه فقط درخواست‌های <span dir="ltr">localhost/127.0.0.1</span> مربوط به <span dir="ltr">TL Studio</span> را می‌پذیرد. در <span dir="ltr">Manifest V3</span> ممکن است <span dir="ltr">Chrome</span>، <span dir="ltr">Service Worker</span> افزونه را بین دو درخواست متوقف کند. اگر با بیدار شدن دوباره، <span dir="ltr">Pairing</span> حافظه‌ای افزونه از بین رفته باشد، <span dir="ltr">TL Studio</span> با همان <span dir="ltr">Token</span> محلی به‌صورت خودکار دوباره <span dir="ltr">Pair</span> می‌کند و همان فرمان انتقال را یک بار تکرار می‌کند. در حالت عادی کاربر نباید دوباره وارد حساب شود.</p>

<h2 dir="rtl" align="right">مرز امنیتی</h2>

<ul dir="rtl">
  <li>کوکی، <span dir="ltr">Session</span> یا اطلاعات ورود <span dir="ltr">Claude</span> توسط <span dir="ltr">TL Studio</span> خوانده، استخراج، ذخیره یا <span dir="ltr">Serialize</span> نمی‌شود.</li>
  <li>درخواست احراز‌شده داخل صفحهٔ عادی <span dir="ltr">claude.ai</span> و با اطلاعات ورود تحت مالکیت مرورگر اجرا می‌شود.</li>
  <li>افزونه فقط فرمان‌های انتقال مدل، یعنی <span dir="ltr">probe</span> و <span dir="ltr">complete</span>، را می‌پذیرد.</li>
  <li>افزونه به فایل‌های پروژه، <span dir="ltr">Terminal</span>، <span dir="ltr">Permission</span>، <span dir="ltr">Session</span> یا <span dir="ltr">Tool</span>های <span dir="ltr">TL Studio</span> دسترسی ندارد.</li>
  <li>چرخهٔ <span dir="ltr">Agent</span>، اجرای ابزارها، مجوزها، فایل‌ها، ترمینال، نشست‌ها و <span dir="ltr">Persistence</span> همچنان در اختیار خود <span dir="ltr">TL Studio</span> باقی می‌ماند.</li>
</ul>

<h2 dir="rtl" align="right">رفع مشکل</h2>

<p dir="rtl" align="right"><strong>افزونه نصب نیست:</strong> گزینهٔ <span dir="ltr">Claude → Web</span> را بزنید. صفحهٔ فروشگاه به‌صورت خودکار باز می‌شود. هنگام نصب، پنجرهٔ اتصال را باز نگه دارید؛ <span dir="ltr">TL Studio</span> افزونه را تشخیص می‌دهد و همان اتصال را خودکار ادامه می‌دهد.</p>

<p dir="rtl" align="right"><strong><span dir="ltr">Claude</span> وارد حساب نیست:</strong> در همان پروفایل <span dir="ltr">Chrome</span> وارد <span dir="ltr">claude.ai</span> شوید و بعد اتصال <span dir="ltr">Web</span> را دوباره برقرار کنید.</p>

<p dir="rtl" align="right"><strong><span dir="ltr">Service Worker</span> افزونه دوباره راه‌اندازی شده:</strong> در حالت عادی کار دستی لازم نیست. <span dir="ltr">TL Studio</span> به‌صورت خودکار دوباره <span dir="ltr">Pair</span> می‌کند و درخواست را یک بار تکرار می‌کند. اگر باز هم خطا باقی ماند، از <span dir="ltr">Provider Settings</span> اتصال <span dir="ltr">Claude → Web</span> را دوباره برقرار کنید.</p>

<p dir="rtl" align="right"><strong>نسخهٔ توسعه:</strong> نسخهٔ <span dir="ltr">Unpacked</span> داخل ریپو برای تست توسعه باقی می‌ماند، اما کاربران عادی باید از نسخهٔ <span dir="ltr">Chrome Web Store</span> استفاده کنند.</p>
