import type { DetectorStatus } from '../types'

const names: Record<string, string> = {
  baseline: '基线',
  'content.injection': '提示注入',
  secret: '密钥(gitleaks)',
  dep: '依赖',
}

export function DetectorStatusList({ list, bare = false }: { list: DetectorStatus[]; bare?: boolean }) {
  // bare=true:仅渲染内部内容(标签 + 状态列表),由父卡片提供容器。
  // bare=false(默认):自带卡片容器(Dashboard 直接放在 grid 里,无外层卡片)。
  const content = (
    <>
      <div className="text-sm text-text-muted mb-3">检测器状态</div>
      <div className="space-y-2">
        {list.map(d => (
          <div key={d.id} className="flex items-center justify-between text-sm">
            <span className="flex items-center gap-2 text-text">
              {/* 状态点:填充圆形 div(sev 色作标记色,非文本),配名称标签读取。 */}
              <span
                className={`inline-block w-2 h-2 rounded-full ${d.available ? 'bg-sev-low' : 'bg-sev-critical'}`}
                aria-hidden="true"
              />
              {names[d.id] ?? d.id}
            </span>
            {!d.available && (
              <span className="text-xs text-text-dim" title={d.reason}>{d.reason}</span>
            )}
          </div>
        ))}
      </div>
    </>
  )
  if (bare) {
    // 嵌入父卡片:用 pt-4 与上方标题分隔,不再自带卡片容器。
    return <div className="pt-4 border-t border-bg-border mt-4">{content}</div>
  }
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl p-5">
      {content}
    </div>
  )
}
