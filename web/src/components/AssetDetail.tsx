import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { apiGet } from '../api/client'
import type { Asset } from '../types'
import { Badge, type BadgeTone } from './Badge'

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
      <div className="bg-bg-card border border-bg-border rounded-xl p-5 space-y-3">
        <div className="flex items-baseline gap-3">
          <h2 className="text-base font-medium" data-testid="asset-detail-name">{asset.name}</h2>
          <Badge tone="neutral">{asset.type}</Badge>
          <Badge tone={`scope-${asset.scope}` as BadgeTone}>{asset.scope}</Badge>
        </div>
        {asset.parse_error && (
          <div className="border border-sev-critical text-sev-critical rounded-md p-2 text-sm">解析错误:{asset.parse_error}</div>
        )}
        <div className="text-xs text-text-dim font-mono break-all">{asset.source_path}</div>
        <div className="grid grid-cols-2 gap-3 text-sm">
          <div><span className="text-text-muted">hash</span><div className="font-mono text-xs break-all">{asset.hash}</div></div>
          <div><span className="text-text-muted">mtime</span><div className="font-mono text-xs">{asset.mtime}</div></div>
        </div>
        {asset.fields && (
          <div>
            <div className="text-text-muted text-sm mb-1">字段</div>
            <pre className="bg-bg border border-bg-border rounded-md p-3 text-xs font-mono overflow-auto max-h-80">{JSON.stringify(asset.fields, null, 2)}</pre>
          </div>
        )}
        {asset.content && (
          <div>
            <div className="text-text-muted text-sm mb-1">内容</div>
            <pre className="bg-bg border border-bg-border rounded-md p-3 text-xs font-mono overflow-auto max-h-96">{asset.content}</pre>
          </div>
        )}
      </div>
    </div>
  )
}
