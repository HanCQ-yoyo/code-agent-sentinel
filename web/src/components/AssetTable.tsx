import { type HTMLAttributes } from 'react'
import { Table, Tag, Typography, Tooltip } from 'antd'
import { StarFilled, StarOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { Asset, Finding, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'
import { resolveDirTag, type DirTag, type DirTagsMap } from '../lib/dirTags'

const rank: Record<Severity, number> = { critical: 4, high: 3, medium: 2, low: 1, info: 0 }

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

const sevLabel: Record<Severity, string> = { critical: '严重', high: '高', medium: '中', low: '低', info: '信息' }

// tagLabel:资产生效标签(相对 .claude 根)。无标签 → 不显示。
function assetTag(a: Asset, defaults: DirTagsMap, overrides: DirTagsMap): DirTag | undefined {
  return resolveDirTag(relativeClaudePath(a.source_path), defaults, overrides)
}

interface AssetTableProps {
  assets: Asset[]
  findings?: Finding[]
  onSelect: (id: string) => void
  // 收藏:set of asset id。空 set = 无收藏。
  favorites: Set<string>
  onToggleFavorite: (id: string) => void
  dirTagsDefaults: DirTagsMap
  dirTagsOverrides: DirTagsMap
}

export function AssetTable({ assets, findings = [], onSelect, favorites, onToggleFavorite, dirTagsDefaults, dirTagsOverrides }: AssetTableProps) {
  const columns: ColumnsType<Asset> = [
    {
      title: '★',
      width: 40,
      // 收藏列:点击星标切换置顶(列排序 + 收藏优先排序由 dataSource 顺序保证)。
      render: (_: unknown, a: Asset) => {
        const fav = favorites.has(a.id)
        return (
          <Tooltip title={fav ? '取消置顶' : '置顶'}>
            <span
              data-testid="fav-toggle"
              onClick={(e) => { e.stopPropagation(); onToggleFavorite(a.id) }}
              style={{ cursor: 'pointer', color: fav ? 'var(--sev-medium)' : 'var(--text-dim)', fontSize: 15, lineHeight: 1, display: 'inline-flex' }}
            >
              {fav ? <StarFilled /> : <StarOutlined />}
            </span>
          </Tooltip>
        )
      },
    },
    {
      title: '名称',
      dataIndex: 'name',
      width: '28%',
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
      title: '标签',
      width: 84,
      render: (_: unknown, a: Asset) => {
        const tag = assetTag(a, dirTagsDefaults, dirTagsOverrides)
        if (!tag) return <span style={{ color: 'var(--text-dim)' }}>—</span>
        const color = tag === 'config' ? 'var(--accent)' : 'var(--text-dim)'
        return (
          <span style={{ fontSize: 11, padding: '0 6px', borderRadius: 8, border: `1px solid ${color}`, color, fontFamily: 'var(--font-sans)' }}>
            {tag === 'config' ? '配置' : '运行时'}
          </span>
        )
      },
    },
    {
      title: '风险',
      width: 84,
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
      // 分页:默认每页 20,显示总数 + 可跳页。资产多时(项目树 align 后可能有几十条)避免长列表。
      pagination={{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], showTotal: (t) => `共 ${t} 条`, size: 'small' }}
      size="middle"
      locale={{ emptyText: '暂无资产' }}
      // 保留 asset-row testid 供 e2e;cast 与 FindingTable onRow 一致(antd Table onRow 无 data-* 索引签名)。
      onRow={(a) => ({ 'data-testid': 'asset-row', onClick: () => onSelect(a.id) }) as HTMLAttributes<HTMLElement>}
      rowClassName={() => 'cursor-pointer'}
    />
  )
}
