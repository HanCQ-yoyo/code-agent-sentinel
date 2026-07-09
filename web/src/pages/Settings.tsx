import { useEffect, useState } from 'react'
import { Card, Tag, Typography, Empty, Badge as AntBadge, Tabs } from 'antd'
import { useStore } from '../store'
import type { DetectorMeta } from '../types'
import { Badge, type BadgeTone } from '../components/Badge'
import { RulesTable } from '../components/RulesTable'

function DetectorCard({ d }: { d: DetectorMeta }) {
  const [open, setOpen] = useState(false)
  return (
    <Card size="small" title={<span style={{ color: 'var(--text)' }}>{d.name}</span>} extra={d.available ? <Tag color="success">可用</Tag> : <Tag color="error">不可用</Tag>}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>引擎</Typography.Text>
          <div style={{ marginTop: 4 }}>
            {(d.engines ?? []).map((e) => (
              <div key={e.name} style={{ fontSize: 13 }}>
                <AntBadge status={e.available ? 'success' : 'error'} />
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
        <div>
          {!d.rules || d.rules.length === 0 ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>由外部扫描引擎内置配置提供</Typography.Text>
          ) : (
            <>
              <Typography.Link style={{ fontSize: 12 }} onClick={() => setOpen(!open)}>{open ? '收起规则' : `展开规则 (${d.rules.length})`}</Typography.Link>
              {open ? (
                <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {d.rules.map((r) => (
                    <div key={r.id} style={{ fontSize: 12 }}>
                      <Badge tone={`sev-${r.severity}` as BadgeTone}>{r.severity}</Badge>
                      <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11, marginLeft: 6 }}>{r.id}</Typography.Text>
                      <Typography.Text type="secondary" style={{ marginLeft: 6 }}>{r.description}</Typography.Text>
                    </div>
                  ))}
                </div>
              ) : null}
            </>
          )}
        </div>
      </div>
    </Card>
  )
}

export default function Settings() {
  const { detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])

  const items = [
    { key: 'detectors', label: '检测器', children: (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {detectors.length === 0 ? <Empty description="暂无检测器" /> : detectors.map((d) => <DetectorCard key={d.id} d={d} />)}
      </div>
    ) },
    { key: 'rules', label: '规则总览', children: <RulesTable detectors={detectors} /> },
    { key: 'about', label: '关于', children: (
      <Card size="small">
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>规则版本随二进制内嵌;密钥检测依赖 gitleaks 子进程,依赖检测依赖 govulncheck/npm-audit。</Typography.Text>
      </Card>
    ) },
  ]

  return (
    <div style={{ maxWidth: 900 }}>
      <Card size="small" style={{ marginBottom: 16 }}><Typography.Text type="secondary">设置(只读)——检测引擎与规则。编辑能力在后续阶段。</Typography.Text></Card>
      <Tabs defaultActiveKey="detectors" items={items} />
    </div>
  )
}
