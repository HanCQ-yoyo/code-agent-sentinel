import { type HTMLAttributes } from 'react'
import { Table, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { Asset, Finding, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'

const rank: Record<Severity, number> = { critical: 4, high: 3, medium: 2, low: 1 }

function maxSev(findings: Finding[], assetId: string): Severity | undefined {
  let best: Severity | undefined
  let bestRank = 0
  for (const f of findings) {
    if (f.asset_id !== assetId) continue
    if ((rank[f.severity] ?? 0) > bestRank) {
      best = f.severity
      bestRank = rank[f.severity]
    }
  }
  return best
}

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低' }

export function AssetTable({ assets, findings = [], onSelect }: { assets: Asset[]; findings?: Finding[]; onSelect: (id: string) => void }) {
  const columns: ColumnsType<Asset> = [
    {
      title: '名称',
      dataIndex: 'name',
      width: '30%',
      render: (name: string, a: Asset) => (
        <span>
          {name}
          {a.parse_error ? <span style={{ color: 'var(--sev-critical)', marginLeft: 6 }} title={a.parse_error}>⚠</span> : null}
        </span>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 110,
      render: (t: string) => <Badge tone="neutral">{t}</Badge>,
    },
    {
      title: 'scope',
      dataIndex: 'scope',
      width: 96,
      render: (s: string) => <Badge tone={`scope-${s}` as BadgeTone}>{s}</Badge>,
    },
    {
      title: '风险',
      width: 96,
      render: (_: unknown, a: Asset) => {
        const sev = maxSev(findings, a.id)
        return sev ? (
          <Badge tone={`sev-${sev}` as BadgeTone}>{sevLabel[sev]}</Badge>
        ) : (
          <Tag style={{ borderStyle: 'dashed', color: 'var(--text-dim)', background: 'transparent' }}>无</Tag>
        )
      },
    },
    {
      title: '路径',
      dataIndex: 'source_path',
      render: (p: string) => (
        <Typography.Text style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-dim)' }} title={p}>
          {relativeClaudePath(p)}
        </Typography.Text>
      ),
    },
  ]

  return (
    <Table<Asset>
      rowKey="id"
      columns={columns}
      dataSource={assets}
      pagination={false}
      size="middle"
      locale={{ emptyText: '暂无资产' }}
      // 保留 asset-row testid 供 e2e;cast 与 FindingTable onRow 一致(antd Table onRow 无 data-* 索引签名)。
      onRow={(a) => ({ 'data-testid': 'asset-row', onClick: () => onSelect(a.id) }) as HTMLAttributes<HTMLElement>}
      rowClassName={() => 'cursor-pointer'}
    />
  )
}
