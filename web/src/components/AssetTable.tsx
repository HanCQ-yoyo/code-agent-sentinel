import { useNavigate } from 'react-router-dom'
import type { Asset, Finding, Severity } from '../types'
import clsx from 'clsx'

// 类型单元格的颜色映射 —— 仅用中性 / accent 文本色,不使用 sev-* 色。
// 原因:sev-high/medium 在浅色面上作为普通文本对比度低于 AA(mark-color 规则:
// 严重度色只用于"标记"——填色圆点/徽章/图标,不用于正文文字)。每行的风险
// 严重度已由"风险"列的填色圆点表达(填色形状 = 标记,OK),类型单元格不应再
// 重复编码严重度。config 类(settings/permissions)用 accent 强调,其余中性。
const typeColor: Record<string, string> = {
  settings: 'text-accent',
  permissions: 'text-accent',
}

export function AssetTable({ assets, findings = [] }: { assets: Asset[]; findings?: Finding[] }) {
  const nav = useNavigate()
  const sevByAsset = new Map<string, Severity>()
  for (const f of findings) {
    const cur = sevByAsset.get(f.asset_id)
    if (!cur || severityRank(f.severity) > severityRank(cur)) sevByAsset.set(f.asset_id, f.severity)
  }
  if (assets.length === 0) {
    return <div className="text-text-muted text-sm p-8 text-center">暂无资产</div>
  }
  return (
    <table className="w-full text-sm">
      <thead className="text-text-muted text-left border-b border-bg-border">
        <tr>
          <th className="p-2 w-24">类型</th><th>名称</th><th className="w-20">scope</th>
          <th className="w-12">风险</th><th>路径</th>
        </tr>
      </thead>
      <tbody>
        {assets.map(a => {
          const sev = sevByAsset.get(a.id)
          return (
            <tr
              key={a.id}
              data-testid="asset-row"
              onClick={() => nav(`/assets/${a.id}`)}
              className="border-b border-bg-border/50 hover:bg-bg-border/30 cursor-pointer"
            >
              <td className={clsx('p-2 font-mono-path text-xs', typeColor[a.type] ?? 'text-text-muted')}>{a.type}</td>
              <td className="p-2">{a.name}{a.parse_error && <span className="text-sev-critical ml-1">⚠</span>}</td>
              <td className="p-2 text-text-muted text-xs">{a.scope}</td>
              <td className="p-2">{sev && <span className="inline-block w-2 h-2 rounded-full" style={{ background: `var(--sev-${sev})` }} title={sev} />}</td>
              <td className="p-2 text-text-dim text-xs font-mono-path truncate max-w-xs" title={a.source_path}>{a.source_path}</td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

function severityRank(s: Severity): number {
  return { critical: 4, high: 3, medium: 2, low: 1 }[s] ?? 0
}
