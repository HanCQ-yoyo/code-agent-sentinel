import { useEffect, useState } from 'react'
import clsx from 'clsx'
import { useStore } from '../store'
import { AssetTable } from '../components/AssetTable'
import { AssetDrawer } from '../components/AssetDrawer'

export default function Assets() {
  const { assets, fetchAssets, scan, error } = useStore()
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
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
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <button onClick={() => setType('')} className={clsx('px-3 py-1 rounded-md text-sm border', !type ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>全部</button>
        {types.map(t => (
          <button key={t} onClick={() => setType(t)} className={clsx('px-3 py-1 rounded-md text-sm border', type === t ? 'border-accent text-accent' : 'border-bg-border text-text-muted')}>{t}</button>
        ))}
        <input
          value={q} onChange={e => setQ(e.target.value)} placeholder="搜索名称/路径…"
          className="ml-auto px-3 py-1 rounded-md bg-bg-card border border-bg-border text-sm w-64"
        />
      </div>
      <div className="text-sm text-text-muted">{list.length} / {all.length} 资产</div>
      <div className="bg-bg-card border border-bg-border rounded-xl overflow-hidden">
        <AssetTable assets={list} findings={scan?.findings} onSelect={setSelected} />
      </div>
      <AssetDrawer id={selected} onClose={() => setSelected(null)} />
    </div>
  )
}
