/**
 * Central theme configuration.
 *
 * ARCHITECTURE-v1 section 10 requires pages to use one theme rather than each
 * choosing its own colours. The values are exposed as CSS custom properties so
 * stylesheets and components read the same source: a component that hard-codes a
 * hex is how a design drifts one page at a time.
 */
export const themeConfig = {
  colors: {
    surface: '#14283b',
    surfaceRaised: '#172d45',
    surfaceSunken: '#0f2436',
    border: '#34516a',
    borderStrong: '#4a6a7f',
    text: '#f0f5f7',
    textMuted: '#9bb0be',
    textFaint: '#7e97a8',
    accent: '#1ad7b0',
    accentSoft: '#58e0c1',
    info: '#31b8e3',
    warning: '#f19a54',
    danger: '#f0a89a',
  },
  radius: { sm: '5px', md: '7px', pill: '99px' },
  space: { xs: '5px', sm: '9px', md: '13px', lg: '18px', xl: '24px' },
} as const;

/** Classification levels ordered least to most sensitive, for consistent tone. */
export const classificationTone: Record<string, 'ok' | 'info' | 'warn' | 'danger'> = {
  public: 'ok',
  internal: 'info',
  confidential: 'warn',
  restricted: 'danger',
};

/** Ingestion and run statuses share one vocabulary of tones across every table. */
export const statusTone: Record<string, 'ok' | 'info' | 'warn' | 'danger'> = {
  ready: 'ok',
  success: 'ok',
  available: 'ok',
  processing: 'info',
  pending: 'info',
  degraded: 'warn',
  'not-ready': 'warn',
  failed: 'danger',
  failure: 'danger',
  unavailable: 'danger',
  denied: 'danger',
};

export function toneFor(map: Record<string, string>, value: string | undefined): string {
  if (!value) return 'info';
  return map[value.toLowerCase()] ?? 'info';
}

/** Applies the theme as CSS custom properties, once, at start-up. */
export function installTheme(): void {
  const root = document.documentElement;
  for (const [name, value] of Object.entries(themeConfig.colors)) {
    root.style.setProperty(`--color-${kebab(name)}`, value);
  }
  for (const [name, value] of Object.entries(themeConfig.radius)) {
    root.style.setProperty(`--radius-${name}`, value);
  }
  for (const [name, value] of Object.entries(themeConfig.space)) {
    root.style.setProperty(`--space-${name}`, value);
  }
}

function kebab(name: string): string {
  return name.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);
}
