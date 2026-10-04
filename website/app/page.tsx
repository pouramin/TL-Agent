import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowRight, Box, Download, FolderCode, Plug, SquareTerminal, UserRound } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'One local application',
    description: 'TL Studio keeps the workspace, Agent, sessions, tools, providers, credentials, Plugins/MCP, Terminal, and Preview inside one local product boundary.',
    icon: Box,
  },
  {
    title: 'Editor-centered workspace',
    description: 'Monaco, Explorer, Search, Sessions, Changes, Terminal, Live Preview, Command Palette, and the Agent share one resizable desktop workspace.',
    icon: SquareTerminal,
  },
  {
    title: 'Native Agent + tools',
    description: 'TL Studio owns the model/tool/model loop, sessions, questions, permissions, semantic events, and the core coding Tool Executor.',
    icon: FolderCode,
  },
  {
    title: 'Providers + account connections',
    description: 'Use direct model protocols, discovery-first custom providers, or supported account-backed connections while credentials stay in the local vault.',
    icon: UserRound,
  },
];

const capabilities = [
  'Monaco Editor',
  'Native Agent',
  'Native Sessions',
  'Tool Executor',
  'Provider Discovery',
  'Provider Connections',
  'Plugins / MCP',
  'Live Preview',
  'Credential Vault',
];

export default function HomePage() {
  return (
    <HomeLayout {...baseOptions('en')}>
      <main className="relative overflow-hidden">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-20 text-center md:pt-28">
          <div className="tl-brand-lockup mb-7">
            <img src={site.logoUrl} alt="TL Studio" />
          </div>
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            Stable v0.6.0 · Development v0.7.0-alpha.1 · Local-first
          </div>
          <h1 className="max-w-5xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            Build software yourself, with AI, or both.
          </h1>
          <p className="mt-7 max-w-3xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            {site.description}
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              Read the docs <ArrowRight className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer">
              <Download className="size-4" /> Download v0.6.0
            </a>
          </div>

          <div className="mt-6 flex max-w-5xl flex-wrap justify-center gap-2">
            {capabilities.map((item) => (
              <span key={item} className="tl-capability-chip rounded-full px-3 py-1 text-sm text-fd-muted-foreground">
                {item}
              </span>
            ))}
          </div>

          <div className="mt-16 w-full max-w-5xl rounded-2xl border bg-fd-card/80 p-5 text-left shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">TL Studio execution path</div>
            <div className="grid gap-3 md:grid-cols-4">
              {['Browser workspace', 'TL Studio local core', 'Native Agent + Tool Executor', 'Providers + MCP tools'].map((label, index) => (
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

        <section className="border-t bg-fd-card/20">
          <div className="mx-auto max-w-6xl px-6 py-14">
            <p className="text-sm font-medium text-fd-muted-foreground">Stable release · v0.6.0</p>
            <h2 className="mt-2 text-3xl font-semibold">A fully native local coding workspace.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              Stable TL Studio owns the workspace, sessions, direct provider calls, routing handoff, questions, permissions, semantic events, native tools, Plugin/MCP execution, and fallback. Router plugins only select a native model; they never take over the Agent runtime.
            </p>
            <Link href="/docs/reference/architecture" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              Explore the architecture <ArrowRight className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-14">
            <p className="text-sm font-medium text-fd-muted-foreground">Development preview · v0.7.0-alpha.1</p>
            <h2 className="mt-2 text-3xl font-semibold">Smarter routing, plugins, and safer updates are stable in 0.6.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              TL Studio v0.6 adds Laya Router, JEV Direct, project-local Graphify graphs, Claude Web Bridge support, responsive Plugin cards, stronger fallback visibility, and a checksum-verified Windows updater while keeping the Native Agent and Tool Executor in control.
            </p>
            <Link href="/docs/guides/custom-providers" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              Explore model connections <ArrowRight className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/20">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">Local trust boundary</p>
            <h2 className="mt-2 text-3xl font-semibold">Your project stays local; external connections are explicit.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              Project files, sessions, permissions, credentials, and workspace state are managed locally. Network access happens through the providers, package distribution, and MCP servers that you explicitly configure.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
