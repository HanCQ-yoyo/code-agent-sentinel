import type { ThemeConfig } from 'antd'
import { theme as antdThemeAlgo } from 'antd'
import { tokens, type Mode } from './tokens'

// Hallmark · genre: modern-minimal · macrostructure family: Workbench · design-system: design.md · designed-as-app
// antd ConfigProvider 映射:把 --color-* 映射到 antd token.color*。组件 token 统一在此定义,
// 替代 index.css 里 !important 压 antd 的做法(view-segmented / card-head-title 等)。
export function antdTheme(mode: Mode): ThemeConfig {
  const t = tokens[mode]
  return {
    algorithm:
      mode === 'dark' ? antdThemeAlgo.darkAlgorithm : antdThemeAlgo.defaultAlgorithm,
    token: {
      colorPrimary: t.accent,
      colorBgBase: t.paper,
      colorBgContainer: t.paper2,
      colorBgElevated: t.paper2,
      colorBgLayout: t.paper,
      colorText: t.ink,
      colorTextSecondary: t.muted,
      colorTextTertiary: t.dim,
      colorTextQuaternary: t.dim,
      colorBorder: t.rule2,
      colorBorderSecondary: t.rule,
      colorError: t.sevCritical,
      colorWarning: t.sevMedium,
      colorSuccess: t.sevLow,
      fontFamily: t.fontSans,
      // 统一字号标准(与 index.css --fs-* 对齐):正文 14 / 小 12 / 大 16 / 标题阶梯。
      fontSize: 14,
      fontSizeSM: 12,
      fontSizeLG: 16,
      fontSizeXL: 20,
      fontSizeHeading1: 28,
      fontSizeHeading2: 24,
      fontSizeHeading3: 20,
      fontSizeHeading4: 18,
      fontSizeHeading5: 16,
      // 圆角统一 8(浅深一致 —— design.md:SOC 看板偏利落,原 light16/dark12 无理由不一致)。
      borderRadius: 8,
      borderRadiusLG: 8,
      borderRadiusSM: 6,
      // focus ring 底色(accent 半透明,design.md microinteractions:即时出现不动画)。
      controlOutline: t.accentSoft,
      wireframe: false,
    },
    components: {
      Layout: { headerBg: t.paper2, siderBg: t.paper2, bodyBg: t.paper },
      Menu: {
        itemBg: t.paper2,
        itemSelectedBg: t.accentSoft,
        itemSelectedColor: t.accent,
        itemColor: t.muted,
        itemHoverColor: t.ink,
        itemHoverBg: t.surface,
        subMenuItemBg: t.surface,
        // 下拉子菜单背景 = paper2(浅色下不再泄漏暗色算法的黑底,修 #6 黑底黑字)。
        groupTitleColor: t.dim,
      },
      // Select / Dropdown 下拉菜单:option 选中态 + hover 态显式设浅色 token,
      // 修 #6「选项 hover 黑底黑字」(根因:antd optionSelectedBg 残留暗色算法派生值)。
      // option 文字色继承别名 colorText(ink);hover/选中背景显式设浅色,避免暗色泄漏。
      Select: {
        optionSelectedColor: t.accent,
        optionSelectedBg: t.accentSoft,
        optionSelectedFontWeight: 600,
        optionActiveBg: t.surface,
      },
      Dropdown: {
        controlItemBgHover: t.surface,
        controlItemBgActive: t.accentSoft,
      },
      // Checkbox 选中态用 accent(修 RescanModal agent 表格勾选框)。
      Checkbox: {
        colorPrimary: t.accent,
        colorPrimaryHover: t.accent,
        borderRadiusSM: 3,
      },
      // Collapse(历史批次分组):header + 内容背景显式设浅色,避免暗色泄漏。
      Collapse: {
        headerBg: t.paper2,
        contentBg: t.paper2,
        contentPadding: '8px 16px',
        headerPadding: '10px 16px',
      },
      Table: {
        headerBg: t.surface,
        borderColor: t.rule,
        rowHoverBg: t.accentSoft,
        // 行选中背景(RescanModal agent 多选):accent-soft,不再泄漏暗色黑底(#7)。
        rowSelectedBg: t.accentSoft,
        rowSelectedHoverBg: t.surface,
      },
      Drawer: { colorBgElevated: t.paper2 },
      Card: { colorBgContainer: t.paper2, colorBorderSecondary: t.rule },
      Tag: { defaultBg: 'transparent' },
      Segmented: {
        // 替代 index.css .view-segmented 的 !important:选中态 accent 实色 + 白字。
        itemSelectedBg: t.accent,
        itemSelectedColor: t.onAccent,
        trackBg: t.surface,
        itemHoverBg: t.surface,
        itemHoverColor: t.accent,
        itemColor: t.muted,
        borderRadius: 6,
        borderRadiusSM: 6,
      },
      Tabs: {
        itemColor: t.muted,
        itemSelectedColor: t.accent,
        itemHoverColor: t.ink,
        inkBarColor: t.accent,
        titleFontSize: 14,
      },
      Button: {
        // CTA voice(design.md):primary accent 实色 + onAccent 字。
        primaryShadow: 'none',
        defaultBorderColor: t.rule2,
        defaultColor: t.ink,
        defaultBg: t.paper2,
      },
      Statistic: {
        titleFontSize: 12,
        contentFontSize: 20,
      },
      Tooltip: { zIndexPopup: 1050 },
      // Switch 选中态继承别名 colorPrimary(accent),无需组件 token 覆盖。
    },
  }
}
