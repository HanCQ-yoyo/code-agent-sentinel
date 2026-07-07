import { useEffect, useState } from 'react'
import clsx from 'clsx'
import { useStore } from '../store'
import { AssetTable } from '../components/AssetTable'
import { AssetTree } from '../components/AssetTree'
import { AssetDrawer } from '../components/AssetDrawer'
import { AssetDetailPanel } from '../components/AssetDetailPanel'

type View = 'list' | 'tree'

export default function Assets() {
  const { assets, fetchAssets, scan, error } = useStore()
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  const [view, setView] = useState<View>('list')
  const [selected, setSelected] = useState<string | null>(null)
  useEffect(() => { fetchAssets() }, [fetchAssets])

  if (!assets && !error) return <div className="text-text-muted p-8">加载中…</div>
  if (error) return <div className="text-sev-critical p-8">加载失败:{error}</div>

  const all = assets?.assets ?? []
  const types = [...new Set(all.map(a => a.type))].sort()
  const list = all.filter(a =>
    (!type || a.type === type) &&
    (!q || a.name.toLowerCase().includes(q.toLowerCase()) || a.source_path.toLowerCase().includes(q.toLowerCase()))
  )
  // 文件树模式:右栏直接从已加载的 all 中取资产,复用 AssetDetailPanel(无需再 fetch)
  const selectedAsset = selected ? all.find(a => a.id === selected) : undefined
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        {/* 视图切换 */}
        <div className="flex border border-bg-border rounded-md overflow-hidden">
          <button onClick={() => setView('list')} className={clsx('px-3 py-1 text-sm', view === 'list' ? 'bg-accent text-white' : 'text-text-muted')}>列表</button>
          <button onClick={() => setView('tree')} className={clsx('px-3 py-1 text-sm', view === 'tree' ? 'bg-accent text-white' : 'text-text-muted')}>文件树</button>
        </div>
        {view === 'list' && (
          <>
            <button onClick={() => setType('')} className={clsx('px-3 py-1 rounded-md text-sm border', !type ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>全部</button>
            {types.map(t => (
              <button key={t} onClick={() => setType(t)} className={clsx('px-3 py-1 rounded-md text-sm border', type === t ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>{t}</button>
            ))}
            <input
              value={q} onChange={e => setQ(e.target.value)} placeholder="搜索名称/路径…"
              className="ml-auto px-3 py-1 rounded-md bg-bg-card border border-bg-border text-sm w-64"
            />
          </>
        )}
      </div>
      <div className="text-sm text-text-muted">{list.length} / {all.length} 资产</div>
      {view === 'list' ? (
        <>
          <div className="bg-bg-card border border-bg-border rounded-xl overflow-hidden">
            <AssetTable assets={list} findings={scan?.findings} onSelect={setSelected} />
          </div>
          <AssetDrawer id={selected} onClose={() => setSelected(null)} />
        </>
      ) : (
        <div className="flex gap-4">
          <div className="flex-1 min-w-0 bg-bg-card border border-bg-border rounded-xl overflow-hidden">
            <div className="p-3"><AssetTree assets={all} findings={scan?.findings} onSelect={setSelected} /></div>
          </div>
          <div className="relative w-full max-w-xl bg-bg-card border border-bg-border rounded-xl overflow-auto p-5 max-h-[80vh] sticky top-4">
            {selectedAsset ? (
              <>
                <button onClick={() => setSelected(null)} className="absolute top-3 right-3 text-text-muted hover:text-text" aria-label="关闭">✕</button>
                <AssetDetailPanel asset={selectedAsset} />
              </>
            ) : (
              <div className="text-text-muted text-sm">选择左侧文件树中的资产查看详情</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
