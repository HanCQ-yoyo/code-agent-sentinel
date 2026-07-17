import { useMemo, useState } from 'react'
import { Tree } from 'antd'
import { FolderOutlined, FileOutlined, FolderOpenOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import type { DataNode } from 'antd/es/tree'
import type { Asset, Finding, Severity, TreeNode } from '../types'
import { Badge, type BadgeTone } from './Badge'
import { resolveDirTag, type DirTag, type DirTagsMap } from '../lib/dirTags'

const rank: Record<Severity, number> = { critical: 4, high: 3, medium: 2, low: 1, info: 0 }

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
  onOpenRaw: (absPath: string) => void
  // 树根绝对路径(全局 ~/.claude 或项目 <proj>/.claude),拼无资产文件的绝对路径用。
  rootAbs: string
  // 标签筛选:null=不过滤;否则只保留该标签的子树(含 untagged 时按"全部"显示)。
  tagFilter: DirTag | null
  dirTagsDefaults: DirTagsMap
  dirTagsOverrides: DirTagsMap
  // 编辑标签回调:点击节点右侧"标签"按钮时弹出菜单。
  onEditTag: (relPath: string, currentTag: DirTag | undefined) => void
  // 受控展开状态:由 Assets 持有(默认全收起 []),提供「全部收起」按钮。
  expandedKeys: React.Key[]
  onExpandedKeysChange: (keys: React.Key[]) => void
}

// nodeTag:节点生效标签(相对 root 的 path → resolveDirTag)。
function nodeTag(n: TreeNode, defaults: DirTagsMap, overrides: DirTagsMap): DirTag | undefined {
  return resolveDirTag(n.path === '.' ? '' : n.path, defaults, overrides)
}

// nodeOrDescendantMatchesTag:节点自身或任一后代命中 tag(用于筛选时保留含目标标签的子树)。
// tagFilter=null 时恒 true。
function subtreeHasTag(n: TreeNode, tag: DirTag, defaults: DirTagsMap, overrides: DirTagsMap): boolean {
  if (nodeTag(n, defaults, overrides) === tag) return true
  for (const c of n.children ?? []) {
    if (subtreeHasTag(c, tag, defaults, overrides)) return true
  }
  return false
}

export function AssetTree({ tree, assets, findings = [], onSelect, onOpenRaw, rootAbs, tagFilter, dirTagsDefaults, dirTagsOverrides, onEditTag, expandedKeys, onExpandedKeysChange }: AssetTreeProps) {
  const { t } = useTranslation()
  const byId = useMemo(() => {
    const m = new Map<string, Asset>()
    for (const a of assets) m.set(a.id, a)
    return m
  }, [assets])

  // path → 节点:目录/文件(无 asset key 时用 path 作 key)的点击分派靠它判断 kind。
  const nodeByKey = useMemo(() => {
    const m = new Map<string, TreeNode>()
    const walk = (n: TreeNode) => {
      m.set(n.path, n)
      for (const c of n.children ?? []) walk(c)
    }
    walk(tree)
    return m
  }, [tree])

  // expandedKeys 受控(由 Assets 持有,默认全收起 []),这里只转发变更。
  const setExpandedKeys = onExpandedKeysChange
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])

  const toggleExpand = (key: React.Key) => {
    setExpandedKeys(
      expandedKeys.includes(key) ? expandedKeys.filter((k) => k !== key) : [...expandedKeys, key],
    )
  }

  // 标签徽标颜色:config=accent, runtime=中性灰。
  const tagBadge = (tag: DirTag | undefined, relPath: string) => {
    if (!tag) return null
    const color = tag === 'config' ? 'var(--accent)' : 'var(--text-dim)'
    return (
      <span
        title={t('assetTree.tagEditTip', { tag })}
        onClick={(e) => { e.stopPropagation(); onEditTag(relPath, tag) }}
        style={{
          marginLeft: 6, fontSize: 10, padding: '0 5px', borderRadius: 8,
          border: `1px solid ${color}`, color, cursor: 'pointer', lineHeight: '16px',
          fontFamily: 'var(--font-sans)', flexShrink: 0,
        }}
      >
        {tag === 'config' ? t('assetTree.tagConfig') : t('assetTree.tagRuntime')}
      </span>
    )
  }

  const toNode = (n: TreeNode): DataNode | null => {
    // 标签筛选:若设了 tagFilter,丢弃自身及后代都不命中的子树。
    if (tagFilter && !subtreeHasTag(n, tagFilter, dirTagsDefaults, dirTagsOverrides)) {
      return null
    }
    const isDir = n.kind === 'dir'
    const isSynthetic = n.kind === 'synthetic'
    const multiAsset = (n.asset_ids?.length ?? 0) > 1
    const scopeColor = n.scope ? `var(--scope-${n.scope})` : undefined
    const tag = nodeTag(n, dirTagsDefaults, dirTagsOverrides)

    const title = isSynthetic ? (
      <span style={{ color: 'var(--text-dim)', fontStyle: 'italic', fontFamily: 'var(--font-mono)' }}>{n.name}</span>
    ) : (
      <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
        {scopeColor && <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: 2, background: scopeColor }} />}
        <span>{n.name}</span>
        {tagBadge(tag, n.path)}
      </span>
    )

    if (isDir) {
      const children = (n.children ?? []).map(toNode).filter(Boolean) as DataNode[]
      return {
        key: n.path,
        title,
        icon: expandedKeys.includes(n.path) ? <FolderOpenOutlined /> : <FolderOutlined />,
        children,
      }
    }
    // file 或 synthetic
    if (multiAsset && n.asset_ids) {
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
      key: singleId ? `asset:${singleId}` : `raw:${n.path}`,
      title: singleId && a ? (
        <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          {sev && <span style={{ display: 'inline-block', width: 6, height: 6, borderRadius: '50%', background: `var(--sev-${sev})` }} />}
          <Badge tone="neutral">{a.type}</Badge>
          <span>{a.name}</span>
          {tagBadge(tag, n.path)}
        </span>
      ) : (
        // 无资产文件:仍显示标签,可点击打开原始内容。
        <span style={{ fontFamily: 'var(--font-mono)', display: 'inline-flex', alignItems: 'center', gap: 6, color: 'var(--text-muted)' }}>
          <span>{n.name}</span>
          {tagBadge(tag, n.path)}
        </span>
      ),
      icon: <FileOutlined />,
      isLeaf: true,
    }
  }

  const treeData: DataNode[] = (() => {
    const root = toNode(tree)
    return root ? [root] : []
  })()

  if (treeData.length === 0) {
    // 标签筛选下整树不命中:显示空提示(不渲染 Tree,避免 antd 空状态样式问题)。
    return <div style={{ padding: 24, color: 'var(--text-dim)', textAlign: 'center' }}>{t('assetTree.noMatch')}</div>
  }

  return (
    <Tree
      treeData={treeData}
      showIcon
      blockNode
      expandedKeys={expandedKeys}
      onExpand={(keys) => setExpandedKeys(keys)}
      selectedKeys={selectedKeys}
      onSelect={(keys) => {
        const k = String(keys[0] ?? '')
        if (!k) return
        // 单资产叶子:直接打开(既有行为)。
        if (k.startsWith('asset:')) {
          const id = k.slice('asset:'.length)
          setSelectedKeys(keys)
          onSelect(id)
          return
        }
        // 无资产文件:打开原始内容。
        if (k.startsWith('raw:')) {
          const rel = k.slice('raw:'.length)
          const n = nodeByKey.get(rel)
          setSelectedKeys(keys)
          if (n) onOpenRaw(absFromRoot(rootAbs, n.path))
          return
        }
        // path-keyed 节点(目录 / 多资产文件):
        const n = nodeByKey.get(k)
        if (!n) return
        if (n.kind === 'dir') {
          // 点击目录名直接展开/折叠,不必点左侧三角。
          toggleExpand(k)
          return
        }
        // 文件节点:多资产开第一个资产。
        const id = n.asset_ids?.[0]
        if (id) {
          setSelectedKeys([`asset:${id}`])
          onSelect(id)
        }
      }}
      style={{ fontFamily: 'var(--font-mono)' }}
    />
  )
}

// absFromRoot:把相对 root 的 path 拼成绝对路径。root 为树根绝对路径(如 /home/x/.claude),
// rel 为节点 path(sessions/3000363.json)。rel 为 "." 或空 → root 本身。
function absFromRoot(root: string, rel: string): string {
  if (!rel || rel === '.') return root
  // rel 用 "/" 分隔(JS 端);root 是 POSIX 绝对路径。直接拼接(Linux 环境)。
  return root.replace(/\/$/, '') + '/' + rel
}
