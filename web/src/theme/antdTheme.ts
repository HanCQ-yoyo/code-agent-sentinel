import type { ThemeConfig } from 'antd'
import { theme as antdThemeAlgo } from 'antd'
import { tokens, type Mode } from './tokens'

export function antdTheme(mode: Mode): ThemeConfig {
  const t = tokens[mode]
  return {
    algorithm:
      mode === 'dark' ? antdThemeAlgo.darkAlgorithm : antdThemeAlgo.defaultAlgorithm,
    token: {
      colorPrimary: t.accent,
      colorBgBase: t.bg,
      colorBgContainer: t.bgCard,
      colorBgElevated: t.surface2,
      colorBgLayout: t.bg,
      colorText: t.text,
      colorTextSecondary: t.textMuted,
      colorTextTertiary: t.textDim,
      colorBorder: t.hairlineStrong,
      colorBorderSecondary: t.bgBorder,
      colorError: t.sevCritical,
      colorWarning: t.sevMedium,
      colorSuccess: t.sevLow,
      fontFamily: t.fontSans,
      // 统一字号标准:正文 14 / 小号 12 / 大号 16 / 标题 16→28 阶梯,与 index.css --fs-* 对齐。
      fontSize: 14,
      fontSizeSM: 12,
      fontSizeLG: 16,
      fontSizeXL: 20,
      fontSizeHeading1: 28,
      fontSizeHeading2: 24,
      fontSizeHeading3: 20,
      fontSizeHeading4: 18,
      fontSizeHeading5: 16,
      borderRadius: mode === 'dark' ? 12 : 16,
    },
    components: {
      Layout: { headerBg: t.bgCard, siderBg: t.bgCard, bodyBg: t.bg },
      Menu: {
        itemBg: t.bgCard,
        itemSelectedBg: t.brandSoft,
        itemSelectedColor: t.accent,
        itemColor: t.textMuted,
      },
      Table: { headerBg: t.surface2, borderColor: t.bgBorder, rowHoverBg: t.brandSoft },
      Drawer: { colorBgElevated: t.bgCard },
      Card: { colorBgContainer: t.bgCard, colorBorderSecondary: t.bgBorder },
      Tag: { defaultBg: 'transparent' },
    },
  }
}
