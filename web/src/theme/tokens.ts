export type Mode = 'light' | 'dark'

export interface TokenSet {
  bg: string
  bgCard: string
  surface2: string
  bgBorder: string
  hairlineStrong: string
  text: string
  textMuted: string
  textDim: string
  accent: string
  brandSoft: string
  sevCritical: string
  sevHigh: string
  sevMedium: string
  sevLow: string
  scopeGlobal: string
  scopeProject: string
  scopeManaged: string
  scopePlugin: string
  fontSans: string
  fontMono: string
}

export const fontSans =
  "'Inter', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif"
export const fontMono =
  "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"

export const tokens: Record<Mode, TokenSet> = {
  light: {
    bg: '#F6F5F1',
    bgCard: '#FFFFFF',
    surface2: '#F2F1EC',
    bgBorder: '#E4E1D8',
    hairlineStrong: '#D2CEC3',
    text: '#1B1A17',
    textMuted: '#5C574F',
    textDim: '#8A857C',
    accent: '#1B6E6A',
    brandSoft: 'rgba(27,110,106,.10)',
    sevCritical: '#C8413A',
    sevHigh: '#C9742E',
    sevMedium: '#B8901F',
    sevLow: '#2E8B4A',
    scopeGlobal: '#1B6E6A',
    scopeProject: '#2E8B4A',
    scopeManaged: '#C9742E',
    scopePlugin: '#6B3A8C',
    fontSans,
    fontMono,
  },
  dark: {
    bg: '#14110F',
    bgCard: '#1C1916',
    surface2: '#221E1A',
    bgBorder: '#2A2622',
    hairlineStrong: '#3A352F',
    text: '#EDE8E0',
    textMuted: '#9A938A',
    textDim: '#6B665F',
    accent: '#2FB8A3',
    brandSoft: 'rgba(47,184,163,.14)',
    sevCritical: '#E0584E',
    sevHigh: '#E89B5A',
    sevMedium: '#E8C547',
    sevLow: '#4FB868',
    scopeGlobal: '#2D4A6B',
    scopeProject: '#2F5D3A',
    scopeManaged: '#6B4A2D',
    scopePlugin: '#4A2D6B',
    fontSans,
    fontMono,
  },
}
