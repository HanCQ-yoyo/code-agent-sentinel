import type { Agent } from '../types'

// agent 展示元数据(图标/标签)。结构性字段(id/root)以后端 /api/agents 为准,
// 展示字段前端持有,避免漂移。未来加 agent 在此扩展映射。
export interface AgentMeta { id: string; label: string; icon: string }

const META: Record<string, AgentMeta> = {
  'claude-code': { id: 'claude-code', label: 'Claude Code', icon: '🤖' },
}

export function agentMeta(a: Agent): AgentMeta {
  return META[a.id] ?? { id: a.id, label: a.name || a.id, icon: '▪' }
}
