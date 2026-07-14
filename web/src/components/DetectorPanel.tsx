import { Typography, Badge as AntBadge, Card } from 'antd'
import type { DetectorMeta } from '../types'
import { Badge } from './Badge'

function ruleCountLabel(d: DetectorMeta): string {
  const n = (d.rules ?? []).length
  if (n > 0) return String(n)
  if (d.engines && d.engines.length > 0) return '外部'
  return '0'
}

function statusBadge(d: DetectorMeta): 'default' | 'success' | 'error' {
  if (!d.enabled) return 'default' // 已禁用
  return d.available ? 'success' : 'error' // 可用 / 不可用
}

// 共享只读检测器面板:chips + 选中详情条。设置页(配置控件叠加在外)与 Dashboard 共用。
// 三态:已禁用(default 灰)/不可用(error)/可用(success)。
export function DetectorPanel({ detectors, selectedId, onSelect }: { detectors: DetectorMeta[]; selectedId?: string; onSelect?: (id: string | undefined) => void }) {
  const selected = selectedId ? detectors.find((d) => d.id === selectedId) : undefined
  const availCount = detectors.filter((d) => d.enabled && d.available).length
  return (
    <div data-testid="detector-chips" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
        <button type="button" onClick={() => onSelect?.(undefined)} aria-pressed={!selectedId}
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 12px', borderRadius: 16, cursor: 'pointer', fontSize: 13, background: !selectedId ? 'var(--brand-soft)' : 'var(--bg-card)', border: `1px solid ${!selectedId ? 'var(--accent)' : 'var(--bg-border)'}`, color: 'var(--text)' }}>
          <span>全部</span>
        </button>
        {detectors.map((d) => (
          <button key={d.id} type="button" onClick={() => onSelect?.(selectedId === d.id ? undefined : d.id)} aria-pressed={selectedId === d.id} data-testid="detector-chip"
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 12px', borderRadius: 16, cursor: 'pointer', fontSize: 13, background: selectedId === d.id ? 'var(--brand-soft)' : 'var(--bg-card)', border: `1px solid ${selectedId === d.id ? 'var(--accent)' : 'var(--bg-border)'}`, color: 'var(--text)' }}>
            <AntBadge status={statusBadge(d)} />
            <span>{d.name}</span>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>{ruleCountLabel(d)}</span>
          </button>
        ))}
      </div>
      {selected ? (
        <Card size="small" style={{ background: 'var(--surface-2)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>状态</Typography.Text>
              <div style={{ marginTop: 4 }}>
                <AntBadge status={statusBadge(selected)} text={!selected.enabled ? '已禁用' : selected.available ? '可用' : '不可用'} />
                {!selected.enabled ? null : !selected.available && selected.reason ? <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>{selected.reason}</Typography.Text> : null}
              </div>
            </div>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>引擎</Typography.Text>
              <div style={{ marginTop: 4 }}>
                {(selected.engines ?? []).map((e) => (
                  <div key={e.name} style={{ fontSize: 13 }}>
                    <AntBadge status={!e.enabled ? 'default' : e.available ? 'success' : 'error'} />
                    <span style={{ color: 'var(--text)', marginLeft: 4 }}>{e.name}</span>
                    <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11, marginLeft: 8 }}>{e.kind}</Typography.Text>
                    {e.enabled && !e.available && e.reason ? <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>{e.reason}</Typography.Text> : null}
                  </div>
                ))}
              </div>
            </div>
            {selected.covers && selected.covers.length > 0 ? (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>覆盖</Typography.Text>
                <div style={{ marginTop: 4, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                  {selected.covers.map((c) => <Badge key={c} tone="neutral">{c}</Badge>)}
                </div>
              </div>
            ) : null}
          </div>
        </Card>
      ) : (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          共 {detectors.length} 个检测器,{availCount} 个可用。
        </Typography.Text>
      )}
    </div>
  )
}
