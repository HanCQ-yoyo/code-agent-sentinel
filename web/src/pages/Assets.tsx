import { useEffect, useState, type ReactNode } from 'react'
import { Card, Segmented, Input, Select, Spin, Alert, Typography, Tabs, Splitter, Modal, Tag, Button, Dropdown } from 'antd'
import { UnorderedListOutlined, ApartmentOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { Asset, PinnedProject, Project } from '../types'
import { AssetTable } from '../components/AssetTable'
import { AssetTree } from '../components/AssetTree'
import { AssetDrawer } from '../components/AssetDrawer'
import { AssetDetailPanel } from '../components/AssetDetailPanel'
import { RawFilePanel } from '../components/RawFilePanel'
import { resolveDirTag, type DirTag } from '../lib/dirTags'
import { relativeClaudePath } from '../lib/path'
import { agentMeta } from '../lib/agents'
import { AgentIcon } from '../components/AgentIcon'

type View = 'list' | 'tree'

// 筛选分组(design.md #1):三类筛选语义不同,用「小字 muted label + 控件」统一结构,
// 但控件形态按语义区分——视图切换(互斥模式,带 icon Segmented)/ 标签筛选(属性过滤,Segmented)/
// 类型筛选(多值过滤,Select 下拉省空间防溢出)。label 区分功能,控件形态区分特点,整体统一。
function FilterGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
      <Typography.Text style={{ fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', textTransform: 'uppercase', letterSpacing: '0.04em', whiteSpace: 'nowrap' }}>
        {label}
      </Typography.Text>
      {children}
    </div>
  )
}

// 收藏/置顶:持久化到后端 config.yaml(跨重启/跨端口)。
// 原用 localStorage,但默认随机端口重启后 origin(host:port)变化 → localStorage 隔离丢失,
// 故改存后端 /api/favorites(与 dir-tags 同模式)。asset id 稳定(scope:type:name:path 哈希)。
const FAV_KEY = 'sentinel_favorites' // 仅作首次迁移:把旧 localStorage 收藏一次性并入后端

export default function Assets() {
  const { t } = useTranslation()
  const {
    assets, fetchAssets, scan, error, tree, projects, activeProjectTab,
    fetchProjects, fetchTree, setActiveProjectTab,
    fetchAgents, agents, fetchDirTags, dirTagsDefaults, dirTagsOverrides,
    saveDirTags, selectedTagFilter, setSelectedTagFilter,
    favorites, fetchFavorites, saveFavorites,
    pinnedProjects, savePinnedProjects,
    detectors, fetchDetectors,
    scanEnabledAgents,
    openRescan,
  } = useStore()
  const [view, setView] = useState<View>('list')
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  // 列表选中(资产 id)→ 打开抽屉;树选中(资产 id)→ 右栏详情。
  const [selected, setSelected] = useState<string | null>(null)
  // 树模式右栏:无资产文件原始内容(path 为绝对路径)。
  const [rawPath, setRawPath] = useState<string | null>(null)
  // 树展开状态受控:默认全收起([])。「全部收起」按钮置空。
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  // 标签编辑弹窗:点击树节点标签徽标时打开,选 配置/运行时/恢复默认。
  const [tagEdit, setTagEdit] = useState<{ relPath: string; current: DirTag | undefined } | null>(null)

  // Task 11:L1 一级 tab = 每个 scanEnabledAgent 一个。activeAgent = 当前选中的 agent ID。
  // 替换 Task 9 的 selectedAgents[0] ?? '' 临时 shim:Assets 页不再依赖全局 selectedAgents,
  // 而是用本页 activeAgent state 显式驱动 fetchAssets/fetchTree/fetchProjects。
  const [activeAgent, setActiveAgent] = useState<string>('')

  // 初始化:scanEnabledAgents 加载后默认选第一个 agent(若有)。
  // 仅在 activeAgent 为空(首次加载)时回填,避免覆盖用户后续切换。
  useEffect(() => {
    if (!activeAgent && scanEnabledAgents.length > 0) {
      setActiveAgent(scanEnabledAgents[0].id)
    }
  }, [scanEnabledAgents, activeAgent])

  // 挂载时拉 agents(派生 scanEnabledAgents)+ 不依赖 agent 的资源(dir-tags/favorites/detectors)。
  // assets/projects/tree 依赖 activeAgent,放在下面的 effect。
  useEffect(() => {
    fetchAgents()
    fetchDirTags()
    fetchFavorites()
    fetchDetectors()
  }, [fetchAgents, fetchDirTags, fetchFavorites, fetchDetectors])

  // Task 11:activeAgent 变化 → 重拉该 agent 的 assets/projects + 重置 L2 到 global。
  // 不含 fetchTree(tree 依赖 activeProjectTab,由下方独立 effect 驱动,避免双重拉取)。
  useEffect(() => {
    if (!activeAgent) return
    fetchAssets(activeAgent)
    fetchProjects(activeAgent)
    // 切 agent → 重置 L2 到 global(切到新 agent 后旧项目 tab 路径可能不存在)。
    setActiveProjectTab({ kind: 'global' })
  }, [activeAgent, fetchAssets, fetchProjects, setActiveProjectTab])

  // 切换 L2 标签页 → 重新拉对应项目的文件树。与上面 activeAgent effect 分离:
  // 原先 activeProjectTab 进了同一 effect deps,导致每次点标签页都重跑 fetchProjects,
  // 后端 map 顺序非确定时标签顺序抖动、选中标签跳到最右。现在 projects 只在 activeAgent 变化时拉,
  // 切标签页只 fetchTree(树随项目变,本就该重拉)。
  // Task 11:fetchTree 显式传 activeAgent(单 agent 或空,不走 agentQuery 的 ?agent=all)。
  useEffect(() => {
    fetchTree(activeProjectTab, activeAgent || undefined)
  }, [fetchTree, activeProjectTab, activeAgent])

  // 一次性迁移:若后端收藏为空但旧 localStorage 有数据,并入后端后清掉本地。
  useEffect(() => {
    if (favorites.length > 0) return
    let raw: string | null = null
    try { raw = localStorage.getItem(FAV_KEY) } catch { return }
    if (!raw) return
    let arr: unknown
    try { arr = JSON.parse(raw) } catch { return }
    if (!Array.isArray(arr)) return
    const ids = arr.filter((x): x is string => typeof x === 'string')
    if (ids.length === 0) return
    saveFavorites(ids).then(() => { try { localStorage.removeItem(FAV_KEY) } catch { /* 忽略 */ } })
  }, [favorites, saveFavorites])

  // 2.2:切换上方标签页 → 关闭详情抽屉(列表)与右栏(树),清选中态。
  // 单独 effect 监听 activeProjectTab 变化,避免与 fetch effect 耦合。
  useEffect(() => {
    setSelected(null)
    setRawPath(null)
  }, [activeProjectTab])

  // Task 11:切 agent(L1 tab)→ 也关闭详情抽屉/右栏,清选中态(与切 L2 同语义)。
  useEffect(() => {
    setSelected(null)
    setRawPath(null)
  }, [activeAgent])

  // Task 10/11:全局根绝对路径用 ACTIVE agent 的 root_dir(后端 Task 7:global tree 根 = 选中 agent 的 root);
  // 项目根 = <tab.path>/.claude。供树拼无资产文件绝对路径。
  // curAgent 从 agents.agents 中按 activeAgent 查找;activeAgent 为空时回退首 agent(兼容初始空态)。
  const curAgent = (agents?.agents ?? []).find(a => a.id === activeAgent) ?? agents?.agents?.[0]
  const rootAbs = activeProjectTab.kind === 'global'
    ? (curAgent?.root_dir ?? '')
    : `${activeProjectTab.path.replace(/\/$/, '')}/.claude`

  const favSet = new Set(favorites)
  const toggleFavorite = (id: string) => {
    const next = new Set(favSet)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    saveFavorites([...next])
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
    const fa = favSet.has(a.id) ? 0 : 1
    const fb = favSet.has(b.id) ? 0 : 1
    if (fa !== fb) return fa - fb
    return a.name.localeCompare(b.name)
  })
  const selectedAsset: Asset | undefined = selected ? all.find((a) => a.id === selected) : undefined

  // Task 17:项目前置。tab 顺序 = 全局 → 置顶(按配置顺序)→ 其余(按名)。
  // 置顶/颜色通过右键菜单(Dropdown contextMenu)操作,全局 tab 不可置顶(无 Dropdown 包裹)。
  const pinnedByPath = new Map(pinnedProjects.map((p) => [p.path, p.color]))
  const pinnedPaths = pinnedProjects.map((p) => p.path)
  // 置顶项目(按配置顺序,且仍被 projects 发现)
  const pinned = projects.filter((p) => pinnedByPath.has(p.path)).sort(
    (a, b) => pinnedPaths.indexOf(a.path) - pinnedPaths.indexOf(b.path)
  )
  const rest = projects.filter((p) => !pinnedByPath.has(p.path)).sort((a, b) => a.name.localeCompare(b.name))

  const togglePin = (path: string, color?: string) => {
    const exists = pinnedProjects.find((p) => p.path === path)
    let next: PinnedProject[]
    if (exists) {
      next = pinnedProjects.filter((p) => p.path !== path)
    } else {
      next = [...pinnedProjects, { path, color: color ?? 'red' }]
    }
    savePinnedProjects(next)
  }
  const setColor = (path: string, color: string) => {
    const next = pinnedProjects.some((p) => p.path === path)
      ? pinnedProjects.map((p) => (p.path === path ? { ...p, color } : p))
      : [...pinnedProjects, { path, color }]
    savePinnedProjects(next)
  }

  const COLORS = [
    { value: 'red', label: t('assets.colorRed') },
    { value: 'orange', label: t('assets.colorOrange') },
    { value: 'gold', label: t('assets.colorGold') },
    { value: 'green', label: t('assets.colorGreen') },
    { value: 'blue', label: t('assets.colorBlue') },
    { value: 'purple', label: t('assets.colorPurple') },
  ]
  // 置顶色走 --cat-* token(design.md:统一 category 色系,不再各写 hex)。red=cat-4/橙=cat-3 偏暖…映射到 6 色 token。
  const colorHex: Record<string, string> = {
    red: 'var(--cat-4)', orange: 'var(--cat-3)', gold: 'var(--cat-3)',
    green: 'var(--cat-2)', blue: 'var(--cat-1)', purple: 'var(--cat-5)',
  }

  const projectTabLabel = (p: Project) => {
    const color = pinnedByPath.get(p.path)
    const menuItems = [
      { key: 'pin', label: color ? t('assets.unpin') : t('assets.pin'), onClick: () => togglePin(p.path) },
      { key: 'color', label: t('assets.setColor'), children: COLORS.map((c) => ({ key: c.value, label: c.label, onClick: () => setColor(p.path, c.value) })) },
      { key: 'rescan', label: t('rescan.rescanThisProject'), onClick: () => openRescan({ type: 'project', path: p.path }) },
    ]
    return (
      <Dropdown trigger={['contextMenu']} menu={{ items: menuItems }}>
        <span data-pinned={color ? 'true' : undefined} style={{ fontWeight: color ? 700 : 400, borderLeft: color ? `3px solid ${colorHex[color]}` : undefined, paddingLeft: color ? 6 : 0 }}>
          {p.name}
        </span>
      </Dropdown>
    )
  }

  const tabItems = [
    { key: 'global', label: t('common.global'), children: null as React.ReactNode },
    ...pinned.map((p) => ({ key: `project:${p.path}`, label: projectTabLabel(p), children: null as React.ReactNode })),
    ...rest.map((p) => ({ key: `project:${p.path}`, label: projectTabLabel(p), children: null as React.ReactNode })),
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
    { label: t('common.all'), value: 'all' as const },
    { label: t('assets.tagConfig'), value: 'config' as const },
    { label: t('assets.tagRuntime'), value: 'runtime' as const },
  ]
  const tagFilterValue = selectedTagFilter ?? 'all'

  // Task 11:L1 agent tab items — 每个 scanEnabledAgent 一个 tab。
  // label = 图标 + 显示名(沿用 AgentMultiSelect/TopBar 风格,agentMeta 处理未知 agent 回退)。
  // 始终渲染 L1(即使单 agent):保持页面结构一致,明确当前资产归属哪个 agent。
  // 仅 scanEnabledAgents 完全为空(未配置任何 agent)时不渲染。
  const agentTabItems = scanEnabledAgents.map((a) => {
    const m = agentMeta(a)
    return { key: a.id, label: <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={a.id} /> {m.label}</span> }
  })
  const showAgentTabs = scanEnabledAgents.length > 0

  // 筛选工具栏行(design.md #2:统一模式——框在结果 Card 内顶部 + 底部 hairline 分隔)。
  // list 与 tree 视图共用(视图切换/标签筛选/类型/搜索都同时作用于列表与树),故提取为常量,
  // 在各视图 Card 顶部渲染一次。原实现筛选在 Card 外,与 History/Findings/RulesTable 不统一。
  const filterRow = (
    <div className="filter-toolbar">
      {/* 视图切换(互斥模式:list/tree 二选一):带 icon 的 Segmented,FilterGroup 标「视图」。 */}
      <FilterGroup label={t('assets.viewLabel')}>
        <Segmented
          className="view-segmented"
          size="small"
          value={view}
          onChange={(v) => setView(v as View)}
          options={[
            { value: 'list', label: <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}><UnorderedListOutlined /> {t('assets.viewList')}</span> },
            { value: 'tree', label: <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}><ApartmentOutlined /> {t('assets.viewTree')}</span> },
          ]}
        />
      </FilterGroup>
      {/* 标签筛选(属性过滤:全部/配置/运行时):Segmented,标「标签」,同时作用于列表与树。 */}
      <FilterGroup label={t('assets.tagLabel')}>
        <Segmented
          size="small"
          value={tagFilterValue}
          onChange={(v) => setSelectedTagFilter(v === 'all' ? null : (v as DirTag))}
          options={tagFilterItems}
        />
      </FilterGroup>
      {view === 'list' ? (
        <>
          {/* 类型筛选(多值过滤):Radio.Group → Select 下拉,省空间防类型多时溢出,标「类型」。 */}
          <FilterGroup label={t('assets.typeLabel')}>
            <Select
              size="small"
              value={type || undefined}
              onChange={(v) => setType(v ?? '')}
              placeholder={t('common.all')}
              style={{ width: 140 }}
              allowClear
              options={types.map((ty) => ({ value: ty, label: ty }))}
            />
          </FilterGroup>
          <Input.Search value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('assets.searchPlaceholder')} style={{ width: 220, marginLeft: 'auto' }} allowClear />
        </>
      ) : (
        // 树模式:全部收起按钮(默认已全收起,展开后可一键收回)。
        <Button size="small" style={{ marginLeft: 'auto' }} onClick={() => setExpandedKeys([])} disabled={expandedKeys.length === 0}>{t('assets.collapseAll')}</Button>
      )}
      <Typography.Text style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)', color: 'var(--color-dim)', fontVariantNumeric: 'tabular-nums' }}>
        {view === 'list' ? t('assets.countList', { shown: list.length, total: tabFiltered.length }) : t('assets.countTree', { total: tabFiltered.length })}
      </Typography.Text>
      {/* 收藏计数提示 */}
      {favorites.length > 0 ? (
        <Tag color="gold" style={{ marginInlineStart: 0 }}>{t('assets.favoritesCount', { count: favorites.length })}</Tag>
      ) : null}
    </div>
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      {showAgentTabs ? (
        <Tabs
          items={agentTabItems}
          activeKey={activeAgent}
          onChange={(key) => setActiveAgent(key)}
          size="small"
          style={{ marginBottom: -8 }}
        />
      ) : null}
      <Tabs
        items={tabItems}
        activeKey={activeTabKey}
        onChange={(key) => {
          if (key === 'global') setActiveProjectTab({ kind: 'global' })
          else if (key.startsWith('project:')) setActiveProjectTab({ kind: 'project', path: key.slice('project:'.length) })
        }}
      />

      {view === 'list' ? (
        <Card>
          {filterRow}
          <AssetTable
            assets={list}
            findings={scan?.findings}
            onSelect={setSelected}
            favorites={favSet}
            onToggleFavorite={toggleFavorite}
            dirTagsDefaults={dirTagsDefaults}
            dirTagsOverrides={dirTagsOverrides}
          />
          <AssetDrawer asset={selectedAsset ?? null} findings={scan?.findings} detectors={detectors} agentID={activeAgent} onClose={() => setSelected(null)} />
        </Card>
      ) : (
        <Card>
          {filterRow}
          <Splitter style={{ height: 'calc(100vh - 280px)', minHeight: 360 }}>
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
                  expandedKeys={expandedKeys}
                  onExpandedKeysChange={setExpandedKeys}
                />
              ) : <Spin />}
            </Card>
          </Splitter.Panel>
          <Splitter.Panel>
            <Card style={{ height: '100%', overflow: 'auto' }}>
              {selectedAsset ? (
                <AssetDetailPanel asset={selectedAsset} findings={scan?.findings} detectors={detectors} agentID={activeAgent} />
              ) : rawPath ? (
                <RawFilePanel path={rawPath} />
              ) : (
                <Typography.Text type="secondary">{t('assets.treeEmptyHint')}</Typography.Text>
              )}
            </Card>
          </Splitter.Panel>
        </Splitter>
        </Card>
      )}
      {/* 标签编辑弹窗:点树节点标签徽标触发,选配置/运行时/恢复默认。 */}
      <Modal
        title={t('assets.editTagTitle')}
        open={tagEdit !== null}
        onCancel={() => setTagEdit(null)}
        footer={null}
        width={420}
      >
        {tagEdit ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', wordBreak: 'break-all' }}>
              {t('assets.pathLabel')}: {tagEdit.relPath || t('assets.rootTag')}
            </Typography.Text>
            <Typography.Text type="secondary">{t('assets.currentTagLabel')}: {tagEdit.current ? (tagEdit.current === 'config' ? t('assets.tagConfig') : t('assets.tagRuntime')) : t('assets.noTagDefault')}</Typography.Text>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'config')} style={{ cursor: 'pointer', borderColor: 'var(--accent)', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--accent)', fontWeight: 500 }}>{t('assets.tagConfig')}</span>
              </Card>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'runtime')} style={{ cursor: 'pointer', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--text-dim)', fontWeight: 500 }}>{t('assets.tagRuntime')}</span>
              </Card>
              <Card size="small" hoverable onClick={() => applyTagEdit(tagEdit.relPath, 'reset')} style={{ cursor: 'pointer', flex: 1, minWidth: 90, textAlign: 'center' }}>
                <span style={{ color: 'var(--text-muted)', fontWeight: 500 }}>{t('assets.resetTag')}</span>
              </Card>
            </div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t('assets.editTagHint')}
            </Typography.Text>
          </div>
        ) : null}
      </Modal>
    </div>
  )
}
