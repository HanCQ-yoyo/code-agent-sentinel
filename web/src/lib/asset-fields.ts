// 各 asset_type 的常见 field 建议表(从内置规则静态收集 + 文档核对)。非穷举:
// field 是自由字符串,用户手输任意值仍合法(后端 validate.go 只校验是 string)。
// 未列出的 asset_type 返回空数组(纯手输)。
export const ASSET_FIELD_SUGGESTIONS: Record<string, string[]> = {
  settings: ['skip_dangerous', 'sandbox_mode', 'env', 'raw'],
  permissions: ['allow', 'deny'],
  hook: ['command', 'matcher', 'event'],
  mcp_server: ['command', 'args', 'env'],
  skill: ['description', 'allowed-tools', 'content'],
  agent: ['name', 'description', 'tools'],
  command: ['command', 'tool', 'args'],
  memory: ['content'],
  script: ['content', 'path'],
  // plugin / keybinding / credential 无强建议 → 不列出(走默认空数组)
}

// 取某 asset_type 的建议;无条目返回空数组。
export function fieldSuggestions(assetType: string): string[] {
  return ASSET_FIELD_SUGGESTIONS[assetType] ?? []
}
