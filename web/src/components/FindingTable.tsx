import { useState } from 'react'
import type { Finding, Severity } from '../types'
import clsx from 'clsx'

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }
const order: Severity[] = ['critical', 'high', 'medium', 'low']

export function FindingTable({ findings }: { findings: Finding[] }) {
  const [sev, setSev] = useState<Severity | ''>('')
  const list = !sev ? findings : findings.filter(f => f.severity === sev)
  const sorted = [...list].sort((a, b) => order.indexOf(a.severity) - order.indexOf(b.severity))
  if (findings.length === 0) return <div className="text-text-muted text-sm p-8 text-center">暂无发现 · 扫描后显示</div>
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <button onClick={() => setSev('')} className={clsx('px-3 py-1 rounded-md text-sm border', !sev ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>全部 {findings.length}</button>
        {order.map(s => {
          const n = findings.filter(f => f.severity === s).length
          return <button key={s} onClick={() => setSev(s)} className={clsx('px-3 py-1 rounded-md text-sm border', sev === s ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>{sevLabel[s]} {n}</button>
        })}
      </div>
      <div className="bg-bg-card border border-bg-border rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead className="text-text-muted text-left border-b border-bg-border">
            <tr><th className="p-2 w-16">级别</th><th>资产</th><th>规则</th><th>说明</th></tr>
          </thead>
          <tbody>
            {sorted.length === 0 ? (
              <tr><td colSpan={4} className="p-8 text-center text-text-muted text-sm">无该级别发现</td></tr>
            ) : sorted.map((f, i) => (
              <tr key={i} data-testid="finding-row" className="border-b border-bg-border/50 align-top">
                <td className="p-2"><span className="inline-block px-2 py-0.5 rounded text-xs" style={{ background: `var(--sev-${f.severity})`, color: '#fff' }}>{sevLabel[f.severity]}</span></td>
                <td className="p-2"><div className="font-medium">{f.asset_name}</div><div className="text-xs text-text-dim font-mono">{f.asset_type}</div></td>
                <td className="p-2 font-mono text-xs text-text-muted">{f.rule_id}</td>
                <td className="p-2"><div>{f.message}</div>{f.evidence && <div className="text-xs text-text-dim font-mono mt-1 break-all">{f.evidence.slice(0, 120)}</div>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
