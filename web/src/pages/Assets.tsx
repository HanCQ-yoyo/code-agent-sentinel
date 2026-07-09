import { useEffect, useMemo, useState } from 'react'
import { Card, Segmented, Input, Radio, Spin, Alert, Typography, Tabs, Splitter, Modal, Tag } from 'antd'
import { useStore } from '../store'
import type { Asset } from '../types'
import { AssetTable } from '../components/AssetTable'
import { AssetTree } from '../components/AssetTree'
import { AssetDrawer } from '../components/AssetDrawer'
import { AssetDetailPanel } from '../components/AssetDetailPanel'
import { RawFilePanel } from '../components/RawFilePanel'
import { resolveDirTag, type DirTag } from '../lib/dirTags'
import { relativeClaudePath } from '../lib/path'

type View = 'list' | 'tree'

// 收藏持久化:asset id 稳定(scope:type:name:path 哈希),存 localStorage 跨会话保留。
const FAV_KEY = 'sentinel_favorites'
function loadFavorites(): Set<string> {
  try {
    const raw = localStorage.getItem(FAV_KEY)
    if (!raw) return new Set()
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? new Set(arr.filter((x) => typeof x === 'string')) : new Set()
  } catch {
    return new Set()
  }
}
function saveFavorites(s: Set<string>) {
  try { localStorage.setItem(FAV_KEY, JSON.stringify([...s])) } catch { /* 忽略配额/隐私模式 */ }
}

export default function Assets() {
  const {
    assets, fetchAssets, scan, error, tree, projects, activeProjectTab,
    fetchProjects, fetchTree, setActiveProjectTab,
    fetchAgents, agents, fetchDirTags, dirTagsDefaults, dirTagsOverrides,
    saveDirTags, selectedTagFilter, setSelectedTagFilter,
  } = useStore()
  const [view, setView] = useState<View>('list')
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  // 列表选中(资产 id)→ 打开抽屉;树选中(资产 id)→ 右栏详情。
  const [selected, setSelected] = useState<string | null>(null)
  // 树模式右栏:无资产文件原始内容(path 为绝对路径)。
  const [rawPath, setRawPath] = useState<string | null>(null)
  const [favorites, setFavorites] = useState<Set<string>>(() => loadFavorites())
  // 标签编辑弹窗:点击树节点标签徽标时打开,选 配置/运行时/恢复默认。
  const [tagEdit, setTagEdit] = useState<{ relPath: string; current: DirTag | undefined } | null>(null)

  useEffect(() => {
    fetchAssets()
    fetchProjects()
    fetchTree(activeProjectTab)
    fetchAgents()
    fetchDirTags()
  }, [fetchAssets, fetchProjects, fetchTree, fetchAgents, fetchDirTags, activeProjectTab])

  // 2.2:切换上方标签页 → 关闭详情抽屉(列表)与右栏(树),清选中态。
  // 单独 effect 监听 activeProjectTab 变化,避免与 fetch effect 耦合。
  useEffect(() => {
    setSelected(null)
    setRawPath(null)
  }, [activeProjectTab])

  // 全局根绝对路径(agents[0].root_dir);项目根 = <tab.path>/.claude。供树拼无资产文件绝对路径。
  const globalRoot = agents?.agents?.[0]?.root_dir ?? ''
  const rootAbs = activeProjectTab.kind === 'global'
    ? globalRoot
    : `${activeProjectTab.path.replace(/\/$/, '')}/.claude`

  const toggleFavorite = (id: string) => {
    setFavorites((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      saveFavorites(next)
      return next
    })
  }

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
  const tagFilter = selectedTagFilter
  // 资产标签:相对 .claude 根(relativeClaudePath)。标签筛选时过滤掉非选中。
  const tagFiltered = tagFilter
    ? tabFiltered.filter((a) => resolveDirTag(relativeClaudePath(a.source_path), dirTagsDefaults, dirTagsOverrides) === tagFilter)
    : tabFiltered
  const searched = tagFiltered.filter((a) =>
    (type === '' || a.type === type) &&
    (q === '' || a.name.toLowerCase().includes(ql) || a.source_path.toLowerCase().includes(ql))
  )
  // 2.7 收藏置顶:收藏的排在前面(再按 name 稳定排序)。
  const list = [...searched].sort((a, b) => {
    const fa = favorites.has(a.id) ? 0 : 1
    const fb = favorites.has(b.id) ? 0 : 1
    if (fa !== fb) return fa - fb
    return a.name.localeCompare(b.name)
  })
  const selectedAsset: Asset | undefined = selected ? all.find((a) => a.id === selected) : undefined

  const tabItems = [
    { key: 'global', label: '全局', children: null as React.ReactNode },
    ...projects.map((p) => ({ key: `project:${p.path}`, label: p.name, children: null as React.ReactNode })),
  ]
  const activeTabKey = activeProjectTab.kind === 'global' ? 'global' : `project:${activeProjectTab.path}`

  // 应用标签编辑:写覆盖(配置/运行时)或删除覆盖(恢复默认)。
  const applyTagEdit = (relPath: string, choice: 'config' | 'runtime' | 'reset') => {
    const next = new Map(Object.entries(dirTagsOverrides))
    if (choice === 'reset') next.delete(relPath)
    else next.set(relPath, choice)
    saveDirTags(Object.fromEntries(next))
    setTagEdit(null)
  }

  const tagFilterItems = [
    { label: '全部', value: 'all' as const },
    { label: '配置', value: 'config' as const },
    { label: '运行时', value: 'runtime' as const },
  ]
  const tagFilterValue = selectedTagFilter ?? 'all'

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
        <Segmented
          className="view-segmented"
          value={view}
          onChange={(v) => setView(v as View)}
          options={[{ value: 'list', label: '列表' }, { value: 'tree', label: '文件树' }]}
        />
        {/* 2.3:标签筛选 Segmented(全部/配置/运行时),同时作用于列表与树。 */}
        <Segmented
          size="small"
          value={tagFilterValue}
          onChange={(v) => setSelectedTagFilter(v === 'all' ? null : (v as DirTag))}
          options={tagFilterItems}
        />
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
        {/* 收藏计数提示 */}
        {favorites.size > 0 ? (
          <Tag color="gold" style={{ marginInlineStart: 'auto' }}>★ {favorites.size} 置顶</Tag>
        ) : null}
      </div>

      {view === 'list' ? (
        <Card>
          <AssetTable
            assets={list}
            findings={scan?.findings}
            onSelect={setSelected}
            favorites={favorites}
            onToggleFavorite={toggleFavorite}
            dirTagsDefaults={dirTagsDefaults}
            dirTagsOverrides={dirTagsOverrides}
          />
          <AssetDrawer asset={selectedAsset ?? null} onClose={() => setSelected(null)} />
        </Card>
      ) : (
        <Splitter style={{ height: 'calc(100vh - 240px)', minHeight: 360 }}>
          {/* 树:窄(原右侧 480 宽与左侧 flex 互换);min 200 防压扁,max 60% 防吃掉详情。
              鼠标拖中间分隔条可调,不再写死。 */}
          <Splitter.Panel defaultSize="34%" min={200} max="60%">
            <Card style={{ height: '100%', overflow: 'auto' }}>
              {tree ? (
                <AssetTree
                  tree={tree}
                  assets={tabFiltered}
                  findings={scan?.findings}
                  onSelect={setSelected}
                  onOpenRaw={(p) => { setSelected(null); setRawPath(p) }}
                  rootAbs={rootAbs}
                  tagFilter={selectedTagFilter}
                  dirTagsDefaults={dirTagsDefaults}
                  dirTagsOverrides={dirTagsOverrides}
                  onEditTag={(rel, cur) => setTagEdit({ relPath: rel, current: cur })}
                />
              ) : <Spin />}
            </Card>
          </Splitter.Panel>
          <Splitter.Panel>
            <Card style={{ height: '100%', overflow: 'auto' }}>
              {selectedAsset ? (
                <AssetDetailPanel asset={selectedAsset} />
              ) : rawPath ? (
                <RawFilePanel path={rawPath} />
              ) : (
                <Typography.Text type="secondary">选择左侧文件树中的资产或文件查看详情</Typography.Text>
              )}
            </Card>
          </Splitter.Panel>
        </Splitter>
      )}
      {/* 标签编辑弹窗:点树节点标签徽标触发,选配置/运行时/恢复默认。 */}
      <Modal
        title="修改目录标签"
        open={tagEdit !== null}
        onCancel={() => setTagEdit(null)}
        footer={null}
        width={420}
      >
        {tagEdit ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', wordBreak: 'break-all' }}>
              路径: {tagEdit.relPath || '(根)'}
            </Typography.Text>
            <Typography.Text type="secondary">当前标签: {tagEdit.current ? (tagEdit.current === 'config' ? '配置' : '运行时') : '无(默认)'}</Typography.Text>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'config')} style={{ cursor: 'pointer', borderColor: 'var(--accent)', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--accent)', fontWeight: 500 }}>配置</span>
              </Card>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'runtime')} style={{ cursor: 'pointer', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--text-dim)', fontWeight: 500 }}>运行时</span>
              </Card>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'reset')} style={{ cursor: 'pointer', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--text-muted)', fontWeight: 500 }}>恢复默认</span>
              </Card>
            </div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              修改后回写配置文件,跨会话保留。子目录/文件继承此标签。
            </Typography.Text>
          </div>
        ) : null}
      </Modal>
    </div>
  )
}
