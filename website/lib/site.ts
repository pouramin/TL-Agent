export const publicBasePath = process.env.NEXT_PUBLIC_BASE_PATH || '';

export const site = {
  name: 'TL Studio',
  tagline: 'Code. Reason. Act.',
  description:
    'A local, standalone coding workspace where the agent, editor, project tools, models, and providers work together — no external IDE required.',
  repoUrl: 'https://github.com/pouramin/TL-Studio',
  releasesUrl: 'https://github.com/pouramin/TL-Studio/releases',
  issuesUrl: 'https://github.com/pouramin/TL-Studio/issues',
  logoUrl: `${publicBasePath}/tl-studio-logo.svg`,
  markUrl: `${publicBasePath}/tl-studio-mark.svg`,
} as const;
