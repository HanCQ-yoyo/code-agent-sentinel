import { useEffect, useState } from 'react'
import { Card, Typography, Empty, Badge as AntBadge, Tabs, Switch, Input, Button } from 'antd'
import { useStore } from '../store'
import type { DetectorMeta, DetectorsConfig } from '../types'
import { Badge, type BadgeTone } from '../components/Badge'
import { RulesTable } from '../components/RulesTable'

// 检测器规则数:有规则返回数字;无规则但有 engines → 外部引擎内置配置;否则 0。
function ruleCountLabel(d: DetectorMeta): string {
  const n = (d.rules ?? []).length
  if (n > 0) return String(n)
  if (d.engines && d.engines.length > 0) return '外部'
  return '0'
}

// 胶囊:可用性圆点 + 名称 + 规则数;选中 accent 高亮。
function DetectorChip({ d, active, onClick }: { d: DetectorMeta; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-testid="detector-chip"
      aria-pressed={active}
      style={{
        display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 12px',
        borderRadius: 16, cursor: 'pointer', fontSize: 13,
        background: active ? 'var(--brand-soft)' : 'var(--bg-card)',
        border: `1px solid ${active ? 'var(--accent)' : 'var(--bg-border)'}`,
        color: 'var(--text)',
      }}
    >
      <AntBadge status={!d.enabled ? 'default' : d.available ? 'success' : 'error'} />
      <span>{d.name}</span>
      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{ruleCountLabel(d)}</span>
    </button>
  )
}

// 选中检测器的紧凑详情条(复用原 DetectorCard 信息):引擎/覆盖/不可用 reason。
function DetectorDetailStrip({ d }: { d: DetectorMeta }) {
  return (
    <Card size="small" style={{ background: 'var(--surface2)' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>引擎</Typography.Text>
          <div style={{ marginTop: 4 }}>
            {(d.engines ?? []).map((e) => (
              <div key={e.name} style={{ fontSize: 13 }}>
                <AntBadge status={!e.enabled ? 'default' : e.available ? 'success' : 'error'} />
                <span style={{ color: 'var(--text)' }}>{e.name}</span>
                <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11, marginLeft: 8 }}>{e.kind}</Typography.Text>
                {!e.available && e.reason ? <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>{e.reason}</Typography.Text> : null}
              </div>
            ))}
          </div>
        </div>
        {d.covers && d.covers.length > 0 ? (
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>覆盖</Typography.Text>
            <div style={{ marginTop: 4, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
              {d.covers.map((c) => <Badge key={c} tone="neutral">{c}</Badge>)}
            </div>
          </div>
        ) : null}
        {!d.available && d.reason ? (
          <Typography.Text type="danger" style={{ fontSize: 12 }}>{d.reason}</Typography.Text>
        ) : null}
      </div>
    </Card>
  )
}

// 检测器配置控件:启用开关 + 二进制路径(rules 仅开关;secret 单二进制;dep 每引擎一行)。
function DetectorConfigControls({ d, draft, setDraft }: { d: DetectorMeta; draft: DetectorsConfig | null; setDraft: (c: DetectorsConfig) => void }) {
  if (!draft) return null
  const patch = (p: Partial<DetectorsConfig>) => setDraft({ ...draft, ...p })
  return (
    <Card size="small" style={{ background: 'var(--surface2)', marginTop: 8 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {d.id === 'rules' ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 13 }}>启用</span>
            <Switch size="small" checked={draft.rules.enabled} onChange={(v) => patch({ rules: { ...draft.rules, enabled: v } })} />
          </div>
        ) : null}
        {d.id === 'secret' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13 }}>启用</span>
              <Switch size="small" checked={draft.secret.enabled} onChange={(v) => patch({ secret: { ...draft.secret, enabled: v } })} />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13, width: 80 }}>二进制路径</span>
              <Input size="small" style={{ flex: 1 }} placeholder="默认 gitleaks" value={draft.secret.binary} onChange={(e) => patch({ secret: { ...draft.secret, binary: e.target.value } })} />
            </div>
          </>
        ) : null}
        {d.id === 'dep' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13 }}>启用</span>
              <Switch size="small" checked={draft.dep.enabled} onChange={(v) => patch({ dep: { ...draft.dep, enabled: v } })} />
            </div>
            {['npm', 'govulncheck'].map((name) => {
              const e = draft.dep.engines[name] ?? { enabled: true, binary: '' }
              return (
                <div key={name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Switch size="small" checked={e.enabled} onChange={(v) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, enabled: v } } } })} />
                  <span style={{ fontSize: 13, width: 100 }}>{name}</span>
                  <Input size="small" style={{ flex: 1 }} placeholder={`默认 ${name}`} value={e.binary} onChange={(ev) => patch({ dep: { ...draft.dep, engines: { ...draft.dep.engines, [name]: { ...e, binary: ev.target.value } } } })} />
                </div>
              )
            })}
          </>
        ) : null}
      </div>
    </Card>
  )
}

export default function Settings() {
  const { detectors, fetchDetectors, detectorConfig, fetchDetectorConfig, saveDetectorConfig } = useStore()
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState<DetectorsConfig | null>(null)
  const [saving, setSaving] = useState(false)
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  useEffect(() => { fetchDetectorConfig() }, [fetchDetectorConfig])
  useEffect(() => { if (detectorConfig) setDraft(detectorConfig) }, [detectorConfig])

  const selected = filter ? detectors.find((d) => d.id === filter) : undefined
  const totalRules = detectors.reduce((n, d) => n + (d.rules ?? []).length, 0)
  const availCount = detectors.filter((d) => d.available).length

  const detectorsAndRules = (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* 胶囊行:检测器粒度统计 + 点击快捷筛选。「全部」清筛选。 */}
      <div data-testid="detector-chips" style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
        <button
          type="button"
          onClick={() => setFilter(undefined)}
          aria-pressed={!filter}
          style={{
            display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 12px',
            borderRadius: 16, cursor: 'pointer', fontSize: 13,
            background: !filter ? 'var(--brand-soft)' : 'var(--bg-card)',
            border: `1px solid ${!filter ? 'var(--accent)' : 'var(--bg-border)'}`,
            color: 'var(--text)',
          }}
        >
          <span>全部</span>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{totalRules}</span>
        </button>
        {detectors.map((d) => (
          <DetectorChip key={d.id} d={d} active={filter === d.id} onClick={() => setFilter(filter === d.id ? undefined : d.id)} />
        ))}
      </div>

      {/* 选中检测器详情条;选「全部」显示摘要。 */}
      {selected ? (
        <>
          <DetectorDetailStrip d={selected} />
          <DetectorConfigControls d={selected} draft={draft} setDraft={setDraft} />
          <div>
            <Button type="primary" size="small" loading={saving} onClick={async () => {
              if (!draft) return
              setSaving(true)
              const ok = await saveDetectorConfig(draft)
              setSaving(false)
              if (!ok) { /* error 已由 wrap 写入 store.error */ }
            }}>保存配置</Button>
          </div>
        </>
      ) : (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          共 {detectors.length} 个检测器,{availCount} 个可用,{totalRules} 条规则。
        </Typography.Text>
      )}

      {/* 规则列表:受胶囊行 detectorFilter 筛选。 */}
      {totalRules === 0 ? <Empty description="暂无规则" /> : <RulesTable detectors={detectors} detectorFilter={filter} />}
    </div>
  )

  const items = [
    { key: 'detectors-rules', label: '规则配置', children: detectorsAndRules },
  ]

  return (
    <div>
      <Tabs items={items} />
    </div>
  )
}
