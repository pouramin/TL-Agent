import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowLeft, Download, FolderCode, Plug, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'یک Application کاملاً Local',
    description: 'TL Studio به‌عنوان Application مستقل خودش اجرا می‌شود؛ زیر Product دیگر Sidecar، Reverse Proxy یا Hidden Fallback Engine وجود ندارد.',
    icon: ServerOff,
  },
  {
    title: 'Workspace با محوریت Editor',
    description: 'Monaco، Explorer، Search، Session، Changes، Terminal، Live Preview، Command Palette و Agent داخل یک Workspace قابل Resize کنار هم هستند.',
    icon: SquareTerminal,
  },
  {
    title: 'Native Agent و Toolها',
    description: 'Agent Loop، Session، Question، Permission، Event و Core Coding Tool Executor متعلق به خود TL Studio هستند.',
    icon: FolderCode,
  },
  {
    title: 'Provider و Plugins/MCP',
    description: 'Modelهای Provider را Discover کنید، Credential را در Vault محلی نگه دارید و Toolهای MCP را وارد همان Native Agent کنید.',
    icon: Plug,
  },
];

const capabilities = [
  'Monaco Editor',
  'Native Agent',
  'Native Sessions',
  'Tool Executor',
  'Plugins / MCP',
  'Provider Discovery',
  'اتصال Native مدل‌ها',
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
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            Stable v0.5.0 · Fully Native Core · Local-first
          </div>
          <h1 className="max-w-5xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            خودت بساز، با AI بساز، یا هردو.
          </h1>
          <p className="mt-7 max-w-3xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            TL Studio یک Browser IDE و Coding-Agent Workspace کاملاً Local است؛ با Native Agent، Project Toolها، Providerها، Plugins/MCP، Terminal و Preview داخل یک Product.
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/fa/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              مستندات <ArrowLeft className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer">
              <Download className="size-4" /> دانلود v0.5.0
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
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">مسیر اجرای TL Studio v0.5</div>
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
            <p className="text-sm font-medium text-fd-muted-foreground">Stable Milestone · v0.5.0</p>
            <h2 className="mt-2 text-3xl font-semibold">TL Studio حالا از ابتدا تا انتها Native اجرا می‌شود.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              Startup عادی فقط خود TL Studio و Browser Assetهای Embedded را بالا می‌آورد. Session، Provider Call، Interactive Question، Permission، Semantic Event، Native Tool و Plugin/MCP همگی پشت Contractهای Local خود TL Studio هستند. Capability پشتیبانی‌نشده به‌صورت شفاف Error می‌دهد و به Local Service مخفی دیگری سپرده نمی‌شود.
            </p>
            <Link href="/fa/docs/reference/architecture" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              معماری پروژه <ArrowLeft className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">Local Trust Boundary</p>
            <h2 className="mt-2 text-3xl font-semibold">Project روی سیستم شما می‌ماند و Connection خارجی صریح است.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              TL Studio، Model Proxy، Workspace Cloud یا Telemetry Backend متعلق به خودش ندارد. Provider و MCP Server مرزهای اعتماد جداگانه‌ای هستند که خود User تنظیم می‌کند؛ File، Session، Permission، Credential و Workspace State به‌صورت Local مدیریت می‌شوند.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
