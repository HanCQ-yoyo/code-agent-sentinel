import type { Agent } from '../types'

// agent 展示元数据(图标/标签)。结构性字段(id/root)以后端 /api/agents 为准,
// 展示字段前端持有,避免漂移。未来加 agent 在此扩展映射。
// 注意: icon 字段仅作回退标识,实际渲染统一用 AgentIcon 组件(品牌 SVG logo)。
export interface AgentMeta { id: string; label: string; icon: string }

const META: Record<string, AgentMeta> = {
  'claude-code': { id: 'claude-code', label: 'Claude Code', icon: '▪' },
  'claude':      { id: 'claude',      label: 'Claude Code', icon: '▪' },
  'codex':       { id: 'codex',       label: 'Codex CLI',  icon: '▪' },
}

export function agentMeta(a: Agent): AgentMeta {
  return META[a.id] ?? { id: a.id, label: a.name || a.id, icon: '▪' }
}

// agentMetaById:仅凭 agent_id 取展示元数据(无完整 Agent 对象时的回退)。
// 已知 agent 走 META;未知 agent 从 agents 列表查 name;都不通则用 id 作 label。
export function agentMetaById(id: string, agents?: Agent[]): AgentMeta {
  if (META[id]) return META[id]
  const a = agents?.find(a => a.id === id)
  if (a) return { id: a.id, label: a.name, icon: '▪' }
  return { id, label: id, icon: '▪' }
}
