import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowLeft, Box, Download, FolderCode, SquareTerminal, UserRound } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'یک محیط کاملاً محلی',
    description: 'محیط کاری، Agent، Sessionها، Toolها، Providerها، Credentialها، Plugins/MCP، Terminal و Preview داخل مرز محصول خود TL Studio باقی می‌مانند.',
    icon: Box,
  },
  {
    title: 'محیط کاری با محوریت Editor',
    description: 'Monaco، Explorer، Search، Sessionها، Changes، Terminal، Live Preview، Command Palette و Agent در یک Workspace قابل Resize کنار هم هستند.',
    icon: SquareTerminal,
  },
  {
    title: 'Agent بومی و Toolها',
    description: 'چرخهٔ مدل و Tool، Sessionها، Questionها، Permissionها، Eventهای معنایی و Tool Executor کدنویسی متعلق به خود TL Studio هستند.',
    icon: FolderCode,
  },
  {
    title: 'Provider و اتصال حساب',
    description: 'می‌توانید از Protocolهای مستقیم، Provider سفارشی با Discovery یا اتصال حساب‌های پشتیبانی‌شده استفاده کنید و Credentialها را در Vault محلی نگه دارید.',
    icon: UserRound,
  },
];

const capabilities = [
  'Monaco Editor',
  'Native Agent',
  'Native Sessions',
  'Tool Executor',
  'Provider Discovery',
  'Account Providers',
  'Plugins / MCP',
  'Live Preview',
  'Credential Vault',
];

export default function PersianHomePage() {
  return (
    <HomeLayout {...baseOptions('fa')}>
      <main dir="rtl" lang="fa" className="relative overflow-hidden text-right">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-20 text-center md:pt-28">
          <div className="tl-brand-lockup mb-7" dir="ltr">
            <img src={site.logoUrl} alt="TL Studio" />
          </div>
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground" dir="ltr">
            Stable v0.5.0 · Development v0.6.0-alpha.1 · Local-first
          </div>
          <h1 className="max-w-5xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            خودت بساز، با هوش مصنوعی بساز، یا هردو.
          </h1>
          <p className="mt-7 max-w-3xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            یک محیط کدنویسی Local-first با Agent بومی، ابزارهای پروژه، Providerها، اتصال حساب، Plugins/MCP، Terminal و Preview.
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/fa/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              مستندات <ArrowLeft className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer" dir="ltr">
              <Download className="size-4" /> Download v0.5.0
            </a>
          </div>

          <div className="mt-6 flex max-w-5xl flex-wrap justify-center gap-2" dir="ltr">
            {capabilities.map((item) => (
              <span key={item} className="tl-capability-chip rounded-full px-3 py-1 text-sm text-fd-muted-foreground">
                {item}
              </span>
            ))}
          </div>

          <div className="mt-16 w-full max-w-5xl rounded-2xl border bg-fd-card/80 p-5 shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">مسیر اجرای TL Studio</div>
            <div className="grid gap-3 md:grid-cols-4" dir="ltr">
              {['Browser Workspace', 'TL Studio Local Core', 'Native Agent + Tool Executor', 'Providers + MCP Tools'].map((label, index) => (
                <div key={label} className="relative">
                  <div className="arch-line rounded-xl px-4 py-5 text-center font-medium">{label}</div>
                  {index < 3 ? <div className="absolute -right-3 top-1/2 hidden -translate-y-1/2 text-fd-muted-foreground md:block">→</div> : null}
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="mx-auto grid max-w-6xl gap-4 px-6 pb-24 md:grid-cols-2">
          {features.map(({ title, description, icon: Icon }) => (
            <article key={title} className="rounded-2xl border bg-fd-card p-6">
              <Icon className="mb-4 size-6" />
              <h2 className="text-lg font-semibold">{title}</h2>
              <p className="mt-2 leading-8 text-fd-muted-foreground">{description}</p>
            </article>
          ))}
        </section>

        <section className="border-t bg-fd-card/20">
          <div className="mx-auto max-w-6xl px-6 py-14">
            <p className="text-sm font-medium text-fd-muted-foreground" dir="ltr">Stable release · v0.5.0</p>
            <h2 className="mt-2 text-3xl font-semibold">یک محیط کدنویسی محلی با اجرای بومی.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              نسخهٔ پایدار، Workspace، Sessionها، Provider Call مستقیم، Questionها، Permissionها، Eventهای معنایی، Toolهای بومی و اجرای Plugins/MCP را در اختیار خود TL Studio نگه می‌دارد. قابلیت پشتیبانی‌نشده با خطای صریح متوقف می‌شود.
            </p>
            <Link href="/fa/docs/reference/architecture" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              معماری پروژه <ArrowLeft className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-14">
            <p className="text-sm font-medium text-fd-muted-foreground" dir="ltr">Development preview · v0.6.0-alpha.1</p>
            <h2 className="mt-2 text-3xl font-semibold">اتصال مدل با حساب کاربری در نسخهٔ 0.6 در حال اضافه‌شدن است.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              خط توسعه برای OpenRouter، Hugging Face و Google/Gemini Flow رسمی Account Login، Model Discovery و Credential مبتنی بر Vault دارد. اتصال ChatGPT/Codex، حساب Claude و GitHub Copilot تا زمانی که قرارداد رسمی آن‌ها با مدل اجرای بومی TL Studio سازگار شود، به‌صورت شفاف Deferred باقی می‌ماند.
            </p>
            <Link href="/fa/docs/guides/custom-providers" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              اتصال مدل‌ها <ArrowLeft className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/20">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">مرز اعتماد محلی</p>
            <h2 className="mt-2 text-3xl font-semibold">پروژه روی سیستم شما می‌ماند و اتصال خارجی صریح است.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              فایل‌ها، Sessionها، Permissionها، Credentialها و Workspace State به‌صورت محلی مدیریت می‌شوند. Network Access فقط از طریق Providerها، توزیع Package و MCP Serverهایی ایجاد می‌شود که خودتان تنظیم کرده‌اید.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
