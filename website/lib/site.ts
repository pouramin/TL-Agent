export const publicBasePath = process.env.NEXT_PUBLIC_BASE_PATH || '';

export const site = {
  name: 'TL Agent',
  tagline: 'Code. Reason. Act.',
  description:
    'A local, standalone coding workspace where the agent, editor, project tools, models, and providers work together — no external IDE required.',
  repoUrl: 'https://github.com/pouramin/TL-Agent',
  releasesUrl: 'https://github.com/pouramin/TL-Agent/releases',
  issuesUrl: 'https://github.com/pouramin/TL-Agent/issues',
  logoUrl: `${publicBasePath}/tl-agent-logo.svg`,
  markUrl: `${publicBasePath}/tl-agent-mark.svg`,
} as const;
