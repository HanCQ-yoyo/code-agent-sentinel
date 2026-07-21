import { type HTMLAttributes } from 'react'
import { Table, Tag, Typography, Tooltip } from 'antd'
import { StarFilled, StarOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import type { ColumnsType } from 'antd/es/table'
import type { Asset, Finding, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { relativeClaudePath } from '../lib/path'
import { resolveDirTag, type DirTag, type DirTagsMap } from '../lib/dirTags'

const rank: Record<Severity, number> = { critical: 4, high: 3, medium: 2, low: 1, info: 0 }

// 统计某资产的全部 finding 数(一个资产可能存在多个风险)。
function findingCount(findings: Finding[], assetId: string): number {
  let n = 0
  for (const f of findings) if (f.asset_id === assetId) n++
  return n
}

// 取该资产最高级别(用于风险数量徽标的配色:按最高级别着色,无风险则中性灰)。
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
  const { t } = useTranslation()
  const columns: ColumnsType<Asset> = [
    {
      title: t('assetTable.colFav'),
      width: 40,
      // 收藏列:点击星标切换置顶(列排序 + 收藏优先排序由 dataSource 顺序保证)。
      render: (_: unknown, a: Asset) => {
        const fav = favorites.has(a.id)
        return (
          <Tooltip title={fav ? t('assetTable.favUnpin') : t('assetTable.favPin')}>
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
      title: t('assetTable.colName'),
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
      title: t('assetTable.colType'),
      dataIndex: 'type',
      width: 110,
      render: (t: string) => <Badge tone="neutral">{t}</Badge>,
    },
    {
      title: t('assetTable.colScope'),
      dataIndex: 'scope',
      width: 96,
      render: (s: string) => <Badge tone={`scope-${s}` as BadgeTone}>{s}</Badge>,
    },
    {
      title: t('assetTable.colTag'),
      width: 84,
      render: (_: unknown, a: Asset) => {
        const tag = assetTag(a, dirTagsDefaults, dirTagsOverrides)
        if (!tag) return <span style={{ color: 'var(--text-dim)' }}>—</span>
        const color = tag === 'config' ? 'var(--accent)' : 'var(--text-dim)'
        return (
          <span style={{ fontSize: 11, padding: '0 6px', borderRadius: 8, border: `1px solid ${color}`, color, fontFamily: 'var(--font-sans)' }}>
            {tag === 'config' ? t('assetTable.tagConfig') : t('assetTable.tagRuntime')}
          </span>
        )
      },
    },
    {
      // 风险数量:一个资产可能存在多个风险,故列改为数量(按最高级别着色),不再只显示最高级别文案。
      // 无风险 → 中性灰「无」标签;有风险 → 带级别色的数字徽标。点击行打开抽屉查看风险列表。
      title: t('assetTable.colRisk'),
      width: 84,
      render: (_: unknown, a: Asset) => {
        const count = findingCount(findings, a.id)
        return count > 0 ? (
          <Badge tone={`sev-${maxSev(findings, a.id)}` as BadgeTone}>{count}</Badge>
        ) : (
          <Tag style={{ borderStyle: 'dashed', color: 'var(--text-dim)', background: 'transparent' }}>{t('assetTable.noRisk')}</Tag>
        )
      },
    },
    {
      title: t('assetTable.colPath'),
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
      // 分页:默认每页 20,显示总数 + 可跳页 + 每页条数选择器。资产多时(项目树 align 后可能有几十条)避免长列表。
      // 用 defaultPageSize(非受控)而非 pageSize(受控):pageSize 作为受控 prop 会在每次渲染覆盖
      // antd usePagination 的内部 innerPagination.pageSize,导致用户用页大小选择器改成 50/100 后
      // 下一次渲染被强制重置回 20(即「每页条数改不动、一直显示 20」bug)。defaultPageSize 仅作初始值,
      // 之后页大小完全交给 antd 内部 state 管理,选择器改动得以保留。
      pagination={{ defaultPageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], showTotal: (total) => t('assetTable.total', { count: total }), size: 'small' }}
      size="middle"
      locale={{ emptyText: t('assetTable.empty') }}
      // 保留 asset-row testid 供 e2e;cast 与 FindingTable onRow 一致(antd Table onRow 无 data-* 索引签名)。
      onRow={(a) => ({ 'data-testid': 'asset-row', onClick: () => onSelect(a.id) }) as HTMLAttributes<HTMLElement>}
      rowClassName={() => 'cursor-pointer'}
    />
  )
}
