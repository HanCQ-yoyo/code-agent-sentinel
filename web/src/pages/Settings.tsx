// web/src/pages/Settings.tsx
import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { Badge, type BadgeTone } from '../components/Badge'
import type { DetectorMeta } from '../types'

export default function Settings() {
  const { detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4 max-w-3xl">
      <div className="bg-bg-card border border-bg-border rounded-xl p-5">
        <h2 className="text-base font-semibold mb-1">设置(只读)</h2>
        <p className="text-sm text-text-muted mb-4">P1 阶段所有配置资产只读。检测器能力与状态如下;缺失的子进程检测器会优雅降级,不阻断扫描。</p>
      </div>
      <div className="space-y-3">
        {detectors.map(d => <DetectorCard key={d.id} d={d} />)}
      </div>
      <div className="bg-bg-card border border-bg-border rounded-xl p-5">
        <h2 className="text-base font-semibold mb-1">关于</h2>
        <div className="text-sm text-text-muted space-y-1">
          <div>规则版本:P1 内置基线 / 提示注入规则集(embedded)</div>
          <div>密钥检测:依赖 gitleaks(缺失时跳过),P2 将重心转移到 MCP/Skills/Scripts 定向检测</div>
        </div>
      </div>
    </div>
  )
}

function DetectorCard({ d }: { d: DetectorMeta }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-medium">{d.name}</h3>
        {d.available
          ? <Badge tone="sev-low">可用</Badge>
          : <Badge tone="sev-critical" >不可用</Badge>}
      </div>
      <div className="mt-3 space-y-1 text-sm">
        <div className="text-text-muted">引擎:</div>
        <div className="ml-4 space-y-0.5">
          {d.engines.map((e, i) => (
            <div key={i} className="flex items-center gap-2">
              <span className={e.available ? 'text-text' : 'text-text-dim'}>{e.name}</span>
              <span className="text-xs text-text-dim">({e.kind})</span>
              {!e.available && e.reason && <span className="text-xs text-text-muted">{e.reason}</span>}
            </div>
          ))}
        </div>
        {d.covers && d.covers.length > 0 && (
          <>
            <div className="text-text-muted">覆盖:</div>
            <div className="ml-4 flex flex-wrap gap-1">
              {d.covers.map(c => <Badge key={c} tone="neutral">{c}</Badge>)}
            </div>
          </>
        )}
        <div className="text-text-muted mt-2">
          规则{d.rules && d.rules.length > 0 ? `(${d.rules.length})` : ''}
          {d.rules && d.rules.length > 0 && (
            <button onClick={() => setOpen(o => !o)} className="ml-2 text-accent text-xs">{open ? '收起 ▴' : '展开 ▾'}</button>
          )}
        </div>
        {open && d.rules && (
          <div className="ml-4 space-y-1">
            {d.rules.map(r => (
              <div key={r.id} className="flex items-start gap-2">
                <Badge tone={`sev-${r.severity}` as BadgeTone}>{r.severity}</Badge>
                <div>
                  <div className="font-mono text-xs text-text-muted">{r.id}</div>
                  <div className="text-xs">{r.description}</div>
                </div>
              </div>
            ))}
          </div>
        )}
        {(!d.rules || d.rules.length === 0) && (
          <div className="ml-4 text-xs text-text-dim">由外部扫描引擎内置配置提供</div>
        )}
      </div>
    </div>
  )
}
