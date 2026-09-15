import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { RootProvider } from 'fumadocs-ui/provider/next';
import SearchDialog from '@/components/search-dialog';
import { LocaleHtmlSync } from '@/components/locale-html-sync';
import { site } from '@/lib/site';
import './global.css';

export const metadata: Metadata = {
  title: {
    default: `${site.name} — Documentation`,
    template: `%s — ${site.name}`,
  },
  description: site.description,
  applicationName: site.name,
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <body className="flex min-h-screen flex-col">
        <RootProvider search={{ SearchDialog }}>
          <LocaleHtmlSync />
          {children}
        </RootProvider>
      </body>
    </html>
  );
}
