export const publicBasePath = process.env.NEXT_PUBLIC_BASE_PATH || '';

export const site = {
  name: 'TL Studio',
  tagline: 'Build locally. Work with AI.',
  description:
    'A local-first coding workspace with a native Agent, project tools, model providers, account integrations, Plugins/MCP, Terminal, and Preview built in.',
  repoUrl: 'https://github.com/pouramin/TL-Studio',
  releasesUrl: 'https://github.com/pouramin/TL-Studio/releases',
  issuesUrl: 'https://github.com/pouramin/TL-Studio/issues',
  sponsorUrl: 'https://buymeacoffee.com/pouramin',
  logoUrl: `${publicBasePath}/tl-studio-logo.svg`,
  markUrl: `${publicBasePath}/tl-studio-mark.svg`,
} as const;
