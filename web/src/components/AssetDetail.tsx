import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { Asset } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

export default function AssetDetail() {
  const { id } = useParams<{ id: string }>()
  const [asset, setAsset] = useState<Asset | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    if (!id) return
    apiGet<Asset>(`/api/assets/${id}`).then(setAsset).catch(e => setErr(String(e)))
  }, [id])
  if (err) return <div className="text-sev-critical p-8">{err}</div>
  if (!asset) return <div className="text-text-muted p-8">加载中…</div>
  return (
    <div className="space-y-4 max-w-4xl">
      <Link to="/assets" className="text-sm text-accent">← 返回资产列表</Link>
      <div className="bg-bg-card border border-bg-border rounded-xl p-5">
        <AssetDetailPanel asset={asset} />
      </div>
    </div>
  )
}
