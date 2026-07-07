import { useNavigate } from 'react-router-dom'
import type { Asset, Finding, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'

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
              <td className="p-2"><Badge tone="neutral">{a.type}</Badge></td>
              <td className="p-2">{a.name}{a.parse_error && <span className="text-sev-critical ml-1">⚠</span>}</td>
              <td className="p-2"><Badge tone={`scope-${a.scope}` as BadgeTone}>{a.scope}</Badge></td>
              <td className="p-2">{sev && <Badge tone={`sev-${sev}` as BadgeTone}>{sev}</Badge>}</td>
              <td className="p-2 text-text-dim text-xs font-mono truncate max-w-xs" title={a.source_path}>{a.source_path}</td>
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
