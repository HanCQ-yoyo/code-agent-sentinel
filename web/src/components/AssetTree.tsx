import { useMemo, useState } from 'react'
import { Tree } from 'antd'
import { FolderOutlined, FileOutlined, FolderOpenOutlined } from '@ant-design/icons'
import type { DataNode } from 'antd/es/tree'
import type { Asset, Finding, Severity, TreeNode } from '../types'
import { Badge, type BadgeTone } from './Badge'

const rank: Record<Severity, number> = { critical: 4, high: 3, medium: 2, low: 1 }

function sevOf(findings: Finding[], assetId: string): Severity | undefined {
  let best: Severity | undefined
  let bestRank = 0
  for (const f of findings) {
    if (f.asset_id !== assetId) continue
    if ((rank[f.severity] ?? 0) > bestRank) {
      best = f.severity
      bestRank = rank[f.severity] ?? 0
    }
  }
  return best
}

interface AssetTreeProps {
  tree: TreeNode
  assets: Asset[]
  findings?: Finding[]
  onSelect: (id: string) => void
}

export function AssetTree({ tree, assets, findings = [], onSelect }: AssetTreeProps) {
  const byId = useMemo(() => {
    const m = new Map<string, Asset>()
    for (const a of assets) m.set(a.id, a)
    return m
  }, [assets])

  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>(() => {
    // 默认展开根 + 第一层目录;synthetic 与 plugins 默认折叠。
    const keys: React.Key[] = [tree.path]
    for (const c of tree.children ?? []) {
      if (c.kind === 'dir' && c.name !== 'plugins') keys.push(c.path)
    }
    return keys
  })
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])

  const toNode = (n: TreeNode): DataNode => {
    const isDir = n.kind === 'dir'
    const isSynthetic = n.kind === 'synthetic'
    // 多资产 file 节点 → 展开为子叶子(每个资产一个);单资产/无资产 → 叶子/目录。
    const multiAsset = (n.asset_ids?.length ?? 0) > 1
    const scopeColor = n.scope ? `var(--scope-${n.scope})` : undefined

    const title = isSynthetic ? (
      <span style={{ color: 'var(--text-dim)', fontStyle: 'italic', fontFamily: 'var(--font-mono)' }}>{n.name}</span>
    ) : (
      <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
        {scopeColor && <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: 2, background: scopeColor }} />}
        {n.name}
      </span>
    )

    if (isDir) {
      return {
        key: n.path,
        title,
        icon: expandedKeys.includes(n.path) ? <FolderOpenOutlined /> : <FolderOutlined />,
        children: (n.children ?? []).map(toNode),
      }
    }
    // file 或 synthetic
    if (multiAsset && n.asset_ids) {
      // 多资产:展开为资产子叶子
      return {
        key: n.path,
        title,
        icon: <FileOutlined />,
        children: n.asset_ids.map(id => {
          const a = byId.get(id)
          const sev = a ? sevOf(findings, a.id) : undefined
          return {
            key: `asset:${id}`,
            title: a ? (
              <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                {sev && <span style={{ display: 'inline-block', width: 6, height: 6, borderRadius: '50%', background: `var(--sev-${sev})` }} />}
                <Badge tone="neutral">{a.type}</Badge>
                <span>{a.name}</span>
              </span>
            ) : <span>{id}</span>,
            icon: null,
            isLeaf: true,
          }
        }),
      }
    }
    // 单资产或无资产文件
    const singleId = n.asset_ids?.[0]
    const a = singleId ? byId.get(singleId) : undefined
    const sev = a ? sevOf(findings, a.id) : undefined
    return {
      key: singleId ? `asset:${singleId}` : n.path,
      title: singleId && a ? (
        <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          {sev && <span style={{ display: 'inline-block', width: 6, height: 6, borderRadius: '50%', background: `var(--sev-${sev})` }} />}
          <Badge tone="neutral">{a.type}</Badge>
          <span>{a.name}</span>
        </span>
      ) : title,
      icon: <FileOutlined />,
      isLeaf: true,
    }
  }

  const treeData: DataNode[] = tree.children ? [toNode(tree)] : []

  return (
    <Tree
      treeData={treeData}
      showIcon
      blockNode
      expandedKeys={expandedKeys}
      onExpand={(keys) => setExpandedKeys(keys)}
      selectedKeys={selectedKeys}
      onSelect={(keys, info) => {
        const k = String(keys[0] ?? '')
        if (k.startsWith('asset:')) {
          const id = k.slice('asset:'.length)
          setSelectedKeys(keys)
          onSelect(id)
        }
      }}
      style={{ fontFamily: 'var(--font-mono)' }}
    />
  )
}
