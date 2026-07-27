// 资产类型 → 显示名 i18n key + 图标 + 顺序的统一映射。
// 抽自 AssetStatTiles.tsx 的本地 specs(8 类型),扩展到 12 类型(Task 12)。
//
// 图标全部从 @ant-design/icons 导入(项目无自定义 IconFont 注册)。
// keybinding 用 KeyOutlined(语义贴切:键盘快捷键),不引入 IconFont。
//
// 消费方:
// - AssetStatTiles:渲染资产数量瓦片(Dashboard 顶部统计)。
// - 其他需要按 asset.type 显示图标/标签的组件可复用 assetTypeMetaByType(type)。
//
// 顺序即渲染顺序:skill → command → agent → plugin → script → hook → mcp_server →
// memory → settings → permissions → keybinding → credential。
// 前 8 个是 Claude Code 的核心配置资产;后 4 个是 Task 12 新增的扩展类型
// (settings/permissions/keybinding/credential 对应 Claude Code 的 settings.json 子结构)。
import type { ReactNode } from 'react'
import {
  ToolOutlined, CodeOutlined, RobotOutlined, AppstoreOutlined, SnippetsOutlined,
  ThunderboltOutlined, ClusterOutlined, FileTextOutlined, SettingOutlined,
  SafetyOutlined, KeyOutlined,
} from '@ant-design/icons'

export interface AssetTypeMeta {
  type: string
  labelKey: string
  icon: ReactNode
}

// 12 资产类型 → labelKey + icon。labelKey 走 i18n(assetStat.*,补缺见 i18n/zh.json + en.json)。
export const ASSET_TYPE_META: AssetTypeMeta[] = [
  { type: 'skill', labelKey: 'assetStat.skill', icon: <ToolOutlined /> },
  { type: 'command', labelKey: 'assetStat.command', icon: <CodeOutlined /> },
  { type: 'agent', labelKey: 'assetStat.agent', icon: <RobotOutlined /> },
  { type: 'plugin', labelKey: 'assetStat.plugin', icon: <AppstoreOutlined /> },
  { type: 'script', labelKey: 'assetStat.script', icon: <SnippetsOutlined /> },
  { type: 'hook', labelKey: 'assetStat.hook', icon: <ThunderboltOutlined /> },
  { type: 'mcp_server', labelKey: 'assetStat.mcp', icon: <ClusterOutlined /> },
  { type: 'memory', labelKey: 'assetStat.memory', icon: <FileTextOutlined /> },
  { type: 'settings', labelKey: 'assetStat.settings', icon: <SettingOutlined /> },
  { type: 'permissions', labelKey: 'assetStat.permissions', icon: <SafetyOutlined /> },
  { type: 'keybinding', labelKey: 'assetStat.keybinding', icon: <KeyOutlined /> },
  { type: 'credential', labelKey: 'assetStat.credential', icon: <KeyOutlined /> },
]

// 按 type 查找单个 meta(未匹配返回 undefined,消费方自行回退)。
export const assetTypeMetaByType = (type: string): AssetTypeMeta | undefined =>
  ASSET_TYPE_META.find((m) => m.type === type)

// 所有资产类型字符串数组(供遍历/校验)。
export const ASSET_TYPES = ASSET_TYPE_META.map((m) => m.type)
