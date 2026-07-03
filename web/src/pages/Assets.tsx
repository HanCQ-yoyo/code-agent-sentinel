import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { AssetList } from '../components/AssetList'
export default function Assets() {
  const { assets, fetchAssets } = useStore()
  const [type, setType] = useState('')
  useEffect(() => { fetchAssets() }, [fetchAssets])
  const list = (assets?.assets ?? []).filter(a => !type || a.type === type)
  const types = [...new Set((assets?.assets ?? []).map(a => a.type))]
  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <button onClick={() => setType('')} className={`px-3 py-1 rounded ${!type ? 'bg-bg-border' : ''}`}>全部</button>
        {types.map(t => <button key={t} onClick={() => setType(t)} className={`px-3 py-1 rounded ${type===t?'bg-bg-border':''}`}>{t}</button>)}
      </div>
      <div className="bg-bg-card border border-bg-border rounded-lg p-2"><AssetList assets={list} /></div>
    </div>
  )
}
