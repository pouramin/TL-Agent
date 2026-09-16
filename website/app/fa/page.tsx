import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowLeft, Download, FolderCode, LockKeyhole, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'کاملاً Local',
    description: 'TL Agent روی کامپیوتر خودتان اجرا می‌شود و مستقیم با Project Local کار می‌کند؛ بدون Cloud Backend یا Database متعلق به TL Agent.',
    icon: ServerOff,
  },
  {
    title: 'Standalone Workspace',
    description: 'بدون وابستگی به VS Code، JetBrains یا Cursor، یک Workspace اختصاصی در مرورگر برای کار با Coding Agent دارید.',
    icon: SquareTerminal,
  },
  {
    title: 'Project-aware Workflow',
    description: 'Folder پروژه را انتخاب کنید، Sessionها را مدیریت کنید، Project Files و Changes را ببینید و هر Conversation را در Context همان Project نگه دارید.',
    icon: FolderCode,
  },
  {
    title: 'Provider Control',
    description: 'از Modelهای Kilo استفاده کنید یا Custom Provider سازگار اضافه کنید. API Key داخل Local Auth Store خود Kilo می‌ماند، نه Browser Storage مربوط به TL Agent.',
    icon: LockKeyhole,
  },
];

const capabilities = [
  'File Attachments',
  'Custom Providers',
  'Retryable Error Recovery',
  'Files + Changes',
  'Interactive Permissions',
  'Theme + Font Size',
  'Loopback-only Runtime',
];

export default function PersianHomePage() {
  return (
    <HomeLayout {...baseOptions('fa')}>
      <main dir="rtl" lang="fa" className="relative overflow-hidden text-right">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-20 text-center md:pt-28">
          <div className="tl-brand-lockup mb-7" dir="ltr">
            <img src={site.logoUrl} alt="TL Agent — Code. Reason. Act." />
          </div>
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            Early Alpha · Local-first · Open Source
          </div>
          <h1 className="max-w-4xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            یک Workspace مستقل و Local برای Coding Agentها، بدون IDE.
          </h1>
          <p className="mt-7 max-w-2xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            TL Agent برای کار با Project، Agent، Model، Tool، Provider، Session، File Attachment و File Changes یک Workspace مستقل داخل مرورگر می‌دهد.
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/fa/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              مستندات <ArrowLeft className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer">
              <Download className="size-4" /> Releases
            </a>
          </div>
          <div className="mt-6 flex max-w-4xl flex-wrap justify-center gap-2" dir="ltr">
            {capabilities.map((item) => (
              <span key={item} className="tl-capability-chip rounded-full px-3 py-1 text-sm text-fd-muted-foreground">
                {item}
              </span>
            ))}
          </div>

          <div className="mt-16 w-full max-w-4xl rounded-2xl border bg-fd-card/80 p-5 shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">TL Agent چطور کار می‌کند؟</div>
            <div className="grid gap-3 md:grid-cols-4" dir="ltr">
              {['Browser Workspace', 'TL Agent Local Layer', 'Agent Runtime', 'Models + Project Tools'].map((label, index) => (
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

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">Independent product · Upstream runtime</p>
            <h2 className="mt-2 text-3xl font-semibold">TL Agent محصول مستقله؛ Kilo Runtime فقط زیرِ کاپوته.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              UX، Project Selection، Local Security، Provider Settings، Packaging و Releaseها متعلق به TL Agent هستند. Kilo در لایه زیرین نقش Agent Runtime را دارد.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
