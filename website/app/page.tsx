import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowRight, Download, FolderCode, Plug, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'One local application',
    description: 'TL Studio runs as its own local application. There is no coding-runtime sidecar, reverse proxy, or hidden fallback engine underneath the product.',
    icon: ServerOff,
  },
  {
    title: 'Editor-centered workspace',
    description: 'Monaco, Explorer, Search, Sessions, Changes, Terminal, Live Preview, Command Palette, and the Agent share one resizable desktop workspace.',
    icon: SquareTerminal,
  },
  {
    title: 'Native Agent + tools',
    description: 'TL Studio owns the model/tool/model loop, sessions, questions, permissions, events, and the core coding Tool Executor.',
    icon: FolderCode,
  },
  {
    title: 'Providers and Plugins/MCP',
    description: 'Discover provider models, keep credentials in the local vault, and expose compatible stdio MCP tools to the same native Agent.',
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
  'JEV / OpenRouter',
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
            Stable v0.5.0 · Fully native core · Local-first
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
              <Download className="size-4" /> Download v0.5.0
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
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">TL Studio v0.5 execution path</div>
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
            <p className="text-sm font-medium text-fd-muted-foreground">Stable milestone · v0.5.0</p>
            <h2 className="mt-2 text-3xl font-semibold">The compatibility-runtime era is over.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              Normal startup is one TL Studio process plus embedded Browser assets. Sessions, provider calls, interactive questions, permissions, semantic events, native tools, and Plugin/MCP execution all live behind TL Studio-owned local contracts. Unsupported model capabilities fail explicitly instead of falling back to another local engine.
            </p>
            <Link href="/docs/reference/architecture" className="mt-5 inline-flex items-center gap-2 font-medium text-fd-primary">
              Explore the architecture <ArrowRight className="size-4" />
            </Link>
          </div>
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">Local trust boundary</p>
            <h2 className="mt-2 text-3xl font-semibold">Your Project stays local; external connections are explicit.</h2>
            <p className="mt-4 max-w-3xl leading-7 text-fd-muted-foreground">
              TL Studio has no product-owned model proxy, workspace cloud, or telemetry backend. Model Providers and MCP servers are separate trust boundaries you configure, while project files, sessions, permissions, credentials, and workspace state remain managed locally.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
