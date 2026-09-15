import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowRight, Download, FolderCode, LockKeyhole, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'Runs on your machine',
    description: 'The launcher, Kilo runtime, selected project, and browser UI stay local. There is no project-owned backend or database.',
    icon: ServerOff,
  },
  {
    title: 'No IDE required',
    description: 'Use Kilo as a coding agent from a standalone browser interface instead of tying the workflow to VS Code, JetBrains, or Cursor.',
    icon: SquareTerminal,
  },
  {
    title: 'Project-aware sessions',
    description: 'Choose a local project, create or reopen Kilo sessions, switch models and agents, and keep the agent scoped to the project you selected.',
    icon: FolderCode,
  },
  {
    title: 'Local security boundary',
    description: 'Both the launcher and Kilo backend bind to loopback. Kilo credentials remain server-side and cross-origin browser requests are rejected.',
    icon: LockKeyhole,
  },
];

export default function HomePage() {
  return (
    <HomeLayout {...baseOptions()}>
      <main className="relative overflow-hidden">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-24 text-center md:pt-32">
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            Early alpha · Local-first · Zero project runtime infrastructure
          </div>
          <h1 className="max-w-4xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            Kilo Code, without living inside an IDE.
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

          <div className="mt-16 w-full max-w-4xl rounded-2xl border bg-fd-card/80 p-5 text-left shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">How the pieces fit together</div>
            <div className="grid gap-3 md:grid-cols-4">
              {['Browser UI', 'Go launcher + proxy', 'kilo serve', 'AI providers + project tools'].map((label, index) => (
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
            <div className="flex flex-col justify-between gap-6 md:flex-row md:items-end">
              <div>
                <p className="text-sm font-medium text-fd-muted-foreground">Current direction</p>
                <h2 className="mt-2 text-3xl font-semibold">A thin local product layer over Kilo, not a fork of Kilo.</h2>
                <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
                  Kilo remains the agent runtime. This project focuses on launch, local security, browser UX, packaging, and a stable bridge to Kilo's headless server API.
                </p>
              </div>
              <a className="inline-flex shrink-0 items-center gap-2 font-medium" href={site.repoUrl} target="_blank" rel="noreferrer">
                View source on GitHub <ArrowRight className="size-4" />
              </a>
            </div>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
