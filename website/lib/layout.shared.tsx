import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { site } from './site';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: site.name,
      url: '/',
    },
    githubUrl: site.repoUrl,
    links: [
      {
        text: 'Documentation',
        url: '/docs',
        active: 'nested-url',
      },
      {
        text: 'Releases',
        url: site.releasesUrl,
        external: true,
      },
    ],
  };
}
