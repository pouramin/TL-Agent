import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowRight, Download, FolderCode, LockKeyhole, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'Local-first by design',
    description: 'TL Agent runs on your computer and works with your local project. There is no TL Agent cloud backend or database in the middle.',
    icon: ServerOff,
  },
  {
    title: 'A workspace of its own',
    description: 'Work with a coding agent from a dedicated browser workspace instead of living inside VS Code, JetBrains, Cursor, or another IDE.',
    icon: SquareTerminal,
  },
  {
    title: 'Project-aware workflow',
    description: 'Pick a local folder, manage sessions, inspect project files and changes, and keep each conversation scoped to the selected project.',
    icon: FolderCode,
  },
  {
    title: 'Provider control without a TL Agent cloud',
    description: 'Use hosted models or add compatible custom providers locally. Provider credentials stay in the local runtime credential store, not TL Agent browser storage.',
    icon: LockKeyhole,
  },
];

const capabilities = [
  'File attachments',
  'Custom providers',
  'Retryable error recovery',
  'Files + changes',
  'Interactive permissions',
  'Theme + font size',
  'Loopback-only runtime',
];

export default function HomePage() {
  return (
    <HomeLayout {...baseOptions('en')}>
      <main className="relative overflow-hidden">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-20 text-center md:pt-28">
          <div className="tl-brand-lockup mb-7">
            <img src={site.logoUrl} alt="TL Agent — Code. Reason. Act." />
          </div>
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            Early alpha · Local-first · Open source
          </div>
          <h1 className="max-w-4xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            Your local coding-agent workspace.
          </h1>
          <p className="mt-7 max-w-2xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            {site.description}
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              Read the docs <ArrowRight className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer">
              <Download className="size-4" /> Download releases
            </a>
          </div>
          <div className="mt-6 flex max-w-4xl flex-wrap justify-center gap-2">
            {capabilities.map((item) => (
              <span key={item} className="tl-capability-chip rounded-full px-3 py-1 text-sm text-fd-muted-foreground">
                {item}
              </span>
            ))}
          </div>

          <div className="mt-16 w-full max-w-4xl rounded-2xl border bg-fd-card/80 p-5 text-left shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">How TL Agent works</div>
            <div className="grid gap-3 md:grid-cols-4">
              {['Browser workspace', 'TL Agent local layer', 'Agent runtime', 'Models + project tools'].map((label, index) => (
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
              <p className="mt-2 leading-7 text-fd-muted-foreground">{description}</p>
            </article>
          ))}
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">Independent product · Local runtime</p>
            <h2 className="mt-2 text-3xl font-semibold">TL Agent is the product layer from end to end.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              Workspace UX, project selection, provider settings, recovery, local security, packaging, and releases all belong to TL Agent. A bundled runtime handles agent execution behind that product boundary.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
