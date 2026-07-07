import { useState } from 'react'
import type { Asset, Finding, Severity } from '../types'
import { Badge, type BadgeTone } from './Badge'

const SCOPE_ORDER = ['global', 'project', 'managed', 'plugin']

// TreeNode:目录节点(children)或文件节点(挂多资产)。
interface DirNode { type: 'dir'; name: string; path: string; children: Map<string, TreeNode> }
interface FileNode { type: 'file'; path: string; assets: Asset[] }
type TreeNode = DirNode | FileNode

// buildTree:把 assets 按 source_path 拆段建树,同路径多资产合并为一个 FileNode。
function buildTree(assets: Asset[]): Map<string, Map<string, TreeNode>> {
  // 外层 key = scope,内层为该 scope 下的根树
  const byScope = new Map<string, Map<string, TreeNode>>()
  for (const a of assets) {
    if (!byScope.has(a.scope)) byScope.set(a.scope, new Map())
    const root = byScope.get(a.scope)!
    const segs = a.source_path.split('/').filter(Boolean)
    if (segs.length === 0) continue
    // 最后一项是文件名,前若干项是目录
    let cur = root
    for (let i = 0; i < segs.length - 1; i++) {
      const name = segs[i]
      let next = cur.get(name)
      if (!next || next.type === 'file') {
        next = { type: 'dir', name, path: segs.slice(0, i + 1).join('/'), children: new Map() }
        cur.set(name, next)
      }
      cur = (next as DirNode).children
    }
    const fileName = segs[segs.length - 1]
    let file = cur.get(fileName)
    if (!file || file.type === 'dir') {
      file = { type: 'file', path: a.source_path, assets: [] }
      cur.set(fileName, file)
    }
    ;(file as FileNode).assets.push(a)
  }
  return byScope
}

function sevOf(findings: Finding[], assetId: string): Severity | undefined {
  let best: Severity | undefined
  for (const f of findings) {
    if (f.asset_id !== assetId) continue
    const rank = { critical: 4, high: 3, medium: 2, low: 1 }[f.severity] ?? 0
    if (!best || rank > ({ critical: 4, high: 3, medium: 2, low: 1 }[best] ?? 0)) best = f.severity
  }
  return best
}

export function AssetTree({ assets, findings = [], onSelect }: { assets: Asset[]; findings?: Finding[]; onSelect: (id: string) => void }) {
  const byScope = buildTree(assets)
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(SCOPE_ORDER)) // 默认展开 scope 分组

  const toggle = (key: string) => setExpanded(s => {
    const n = new Set(s)
    if (n.has(key)) n.delete(key); else n.add(key)
    return n
  })

  return (
    <div className="text-sm font-mono select-none">
      {SCOPE_ORDER.filter(s => byScope.has(s)).map(scope => {
        const key = `scope:${scope}`
        const open = expanded.has(key)
        return (
          <div key={scope}>
            <div className="flex items-center gap-1 px-2 py-1 cursor-pointer hover:bg-bg-border/30 text-text-muted" onClick={() => toggle(key)}>
              <span className="w-3">{open ? '▾' : '▸'}</span>
              <Badge tone={`scope-${scope}` as BadgeTone}>{scope}</Badge>
            </div>
            {open && (
              <div className="ml-4 border-l border-bg-border">
                <SubTree nodes={byScope.get(scope)!} depth={0} expanded={expanded} toggle={toggle} findings={findings} onSelect={onSelect} />
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function SubTree({ nodes, depth, expanded, toggle, findings, onSelect }: {
  nodes: Map<string, TreeNode>; depth: number; expanded: Set<string>; toggle: (k: string) => void
  findings: Finding[]; onSelect: (id: string) => void
}) {
  return <>{Array.from(nodes.entries()).sort(([a], [b]) => a.localeCompare(b)).map(([name, node]) => {
    if (node.type === 'dir') {
      const key = `dir:${node.path}`
      const open = expanded.has(key)
      return (
        <div key={key}>
          <div className="flex items-center gap-1 px-2 py-1 cursor-pointer hover:bg-bg-border/30" style={{ paddingLeft: depth * 12 + 8 }} onClick={() => toggle(key)}>
            <span className="w-3">{open ? '▾' : '▸'}</span>
            <span className="text-text">{name}/</span>
          </div>
          {open && <SubTree nodes={node.children} depth={depth + 1} expanded={expanded} toggle={toggle} findings={findings} onSelect={onSelect} />}
        </div>
      )
    }
    // file node:挂多资产,点资产 onSelect
    return (
      <div key={`file:${node.path}`} style={{ paddingLeft: depth * 12 + 8 }}>
        <div className="px-2 py-1 text-text-muted">{name}</div>
        {node.assets.map(a => {
          const sev = sevOf(findings, a.id)
          return (
            <div key={a.id} onClick={() => onSelect(a.id)} className="flex items-center gap-2 px-2 py-0.5 cursor-pointer hover:bg-bg-border/30" style={{ paddingLeft: depth * 12 + 24 }}>
              {sev && <span className="inline-block w-1.5 h-1.5 rounded-full" style={{ background: `var(--sev-${sev})` }} />}
              <Badge tone="neutral">{a.type}</Badge>
              <span className="text-text">{a.name}</span>
            </div>
          )
        })}
      </div>
    )
  })}</>
}
