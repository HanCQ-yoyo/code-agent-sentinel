import { useEffect, useState } from 'react'
import { Card, Segmented, Input, Radio, Spin, Alert, Typography, Tabs } from 'antd'
import { useStore } from '../store'
import type { Asset } from '../types'
import { AssetTable } from '../components/AssetTable'
import { AssetTree } from '../components/AssetTree'
import { AssetDrawer } from '../components/AssetDrawer'
import { AssetDetailPanel } from '../components/AssetDetailPanel'

type View = 'list' | 'tree'

export default function Assets() {
  const { assets, fetchAssets, scan, error, tree, projects, activeProjectTab, fetchProjects, fetchTree, setActiveProjectTab } = useStore()
  const [view, setView] = useState<View>('list')
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string | null>(null)

  useEffect(() => {
    fetchAssets()
    fetchProjects()
    fetchTree(activeProjectTab)
  }, [fetchAssets, fetchProjects, fetchTree, activeProjectTab])

  if (!assets) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  const all = assets.assets

  // 按 activeProjectTab 过滤(纯视图):全局 tab = scope∈{global,plugin};
  // 项目 tab = source_path 在 <path>/ 下(与后端 project scope 语义一致,
  // 含项目根资产如 .mcp.json,不仅限于 <path>/.claude/)。
  const tabFiltered = all.filter((a) => {
    if (activeProjectTab.kind === 'global') {
      return a.scope === 'global' || a.scope === 'plugin'
    }
    return a.source_path.startsWith(`${activeProjectTab.path}/`)
  })

  const types = [...new Set(tabFiltered.map((a) => a.type))].sort()
  const ql = q.toLowerCase()
  const list = tabFiltered.filter((a) =>
    (type === '' || a.type === type) &&
    (q === '' || a.name.toLowerCase().includes(ql) || a.source_path.toLowerCase().includes(ql))
  )
  const selectedAsset: Asset | undefined = selected ? all.find((a) => a.id === selected) : undefined

  const tabItems = [
    { key: 'global', label: '全局', children: null as React.ReactNode },
    ...projects.map((p) => ({ key: `project:${p.path}`, label: p.name, children: null as React.ReactNode })),
  ]
  const activeTabKey = activeProjectTab.kind === 'global' ? 'global' : `project:${activeProjectTab.path}`

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {error ? <Alert type="error" message="加载失败" description={error} showIcon /> : null}
      <Tabs
        items={tabItems}
        activeKey={activeTabKey}
        onChange={(key) => {
          if (key === 'global') setActiveProjectTab({ kind: 'global' })
          else if (key.startsWith('project:')) setActiveProjectTab({ kind: 'project', path: key.slice('project:'.length) })
        }}
      />
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <Segmented value={view} onChange={(v) => setView(v as View)} options={[{ value: 'list', label: '列表' }, { value: 'tree', label: '文件树' }]} />
        {view === 'list' ? (
          <>
            <Radio.Group value={type} onChange={(e) => setType(e.target.value)} size="small">
              <Radio.Button value="">全部</Radio.Button>
              {types.map((t) => <Radio.Button key={t} value={t}>{t}</Radio.Button>)}
            </Radio.Group>
            <Input.Search value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索名称或路径" style={{ width: 240, marginLeft: 'auto' }} allowClear />
          </>
        ) : null}
        <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)' }}>
          {view === 'list' ? `${list.length} / ${tabFiltered.length} 资产` : `${tabFiltered.length} 资产`}
        </Typography.Text>
      </div>

      {view === 'list' ? (
        <Card>
          <AssetTable assets={list} findings={scan?.findings} onSelect={setSelected} />
          <AssetDrawer asset={selectedAsset ?? null} onClose={() => setSelected(null)} />
        </Card>
      ) : (
        <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
          <Card style={{ flex: 1 }}>
            {tree ? <AssetTree tree={tree} assets={tabFiltered} findings={scan?.findings} onSelect={setSelected} /> : <Spin />}
          </Card>
          <Card style={{ width: 480, position: 'sticky', top: 16, maxHeight: '80vh', overflow: 'auto' }}>
            {selectedAsset ? <AssetDetailPanel asset={selectedAsset} /> : <Typography.Text type="secondary">选择左侧文件树中的资产查看详情</Typography.Text>}
          </Card>
        </div>
      )}
    </div>
  )
}
