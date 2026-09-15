import Link from 'next/link';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { ArrowLeft, Download, FolderCode, LockKeyhole, ServerOff, SquareTerminal } from 'lucide-react';
import { baseOptions } from '@/lib/layout.shared';
import { site } from '@/lib/site';

const features = [
  {
    title: 'طراحی‌شده برای اجرای محلی',
    description: 'TL Agent روی کامپیوتر خودتان اجرا می‌شود و مستقیماً با پروژه محلی شما کار می‌کند؛ بدون بک‌اند ابری یا دیتابیس متعلق به TL Agent.',
    icon: ServerOff,
  },
  {
    title: 'محیط کاری مستقل',
    description: 'بدون وابستگی به VS Code، JetBrains یا Cursor، از یک محیط اختصاصی در مرورگر برای کار با Coding Agent استفاده کنید.',
    icon: SquareTerminal,
  },
  {
    title: 'متمرکز روی پروژه شما',
    description: 'پوشه پروژه را انتخاب کنید، Session بسازید، Agent و Model را تغییر دهید و تغییرات فایل‌ها را داخل همان محیط ببینید.',
    icon: FolderCode,
  },
  {
    title: 'مرز امنیتی محلی',
    description: 'TL Agent فقط روی loopback اجرا می‌شود، اطلاعات Runtime را سمت سرور محلی نگه می‌دارد و برای عملیات حساس Agent از شما اجازه می‌گیرد.',
    icon: LockKeyhole,
  },
];

export default function PersianHomePage() {
  return (
    <HomeLayout {...baseOptions('fa')}>
      <main dir="rtl" lang="fa" className="relative overflow-hidden text-right">
        <div className="hero-grid pointer-events-none absolute inset-0 -z-10 opacity-60" />
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pb-20 pt-24 text-center md:pt-32">
          <div className="mb-6 rounded-full border bg-fd-card/70 px-4 py-1.5 text-sm text-fd-muted-foreground">
            نسخه آزمایشی · Local-first · متن‌باز
          </div>
          <h1 className="max-w-4xl text-balance text-5xl font-bold tracking-tight md:text-7xl">
            محیط محلی شما برای کار با Coding Agent.
          </h1>
          <p className="mt-7 max-w-2xl text-balance text-lg leading-8 text-fd-muted-foreground md:text-xl">
            TL Agent یک محیط مستقل و محلی برای کار با پروژه‌ها، Agentها، Modelها، ابزارها، Sessionها و تغییرات فایل از داخل مرورگر است؛ بدون نیاز به IDE.
          </p>
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            <Link href="/fa/docs" className="inline-flex items-center gap-2 rounded-lg bg-fd-primary px-5 py-3 font-medium text-fd-primary-foreground">
              مطالعه مستندات <ArrowLeft className="size-4" />
            </Link>
            <a href={site.releasesUrl} className="inline-flex items-center gap-2 rounded-lg border bg-fd-card px-5 py-3 font-medium" target="_blank" rel="noreferrer">
              <Download className="size-4" /> دریافت نسخه‌ها
            </a>
          </div>

          <div className="mt-16 w-full max-w-4xl rounded-2xl border bg-fd-card/80 p-5 shadow-sm md:p-8">
            <div className="mb-5 text-sm font-medium text-fd-muted-foreground">TL Agent چطور کار می‌کند؟</div>
            <div className="grid gap-3 md:grid-cols-4" dir="ltr">
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
              <p className="mt-2 leading-8 text-fd-muted-foreground">{description}</p>
            </article>
          ))}
        </section>

        <section className="border-t bg-fd-card/35">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <p className="text-sm font-medium text-fd-muted-foreground">محصول مستقل، Runtime بالادستی</p>
            <h2 className="mt-2 text-3xl font-semibold">TL Agent محصول ماست؛ Kilo فقط Runtime زیرِ آن را تأمین می‌کند.</h2>
            <p className="mt-4 max-w-3xl leading-8 text-fd-muted-foreground">
              هویت محصول، محیط کار، امنیت محلی، انتخاب پروژه، بسته‌بندی و Releaseها متعلق به TL Agent است. Kilo یک وابستگی Runtime در لایه زیرین باقی می‌ماند.
            </p>
          </div>
        </section>
      </main>
    </HomeLayout>
  );
}
