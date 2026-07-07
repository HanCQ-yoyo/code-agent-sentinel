import { useEffect, useState } from 'react'
import { apiGet } from '../api/client'
import type { Asset } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

// AssetDrawer 是右侧抽屉:id 非 null 时打开,内部 fetch 资产并渲染 Panel。
export function AssetDrawer({ id, onClose }: { id: string | null; onClose: () => void }) {
  const [asset, setAsset] = useState<Asset | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!id) { setAsset(null); setErr(''); return }
    setAsset(null); setErr('')
    apiGet<Asset>(`/api/assets/${id}`).then(setAsset).catch(e => setErr(String(e)))
  }, [id])

  useEffect(() => {
    if (!id) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [id, onClose])

  if (!id) return null
  return (
    <div className="fixed inset-0 z-40 flex justify-end" data-testid="asset-drawer">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative w-full max-w-xl bg-bg-card border-l border-bg-border overflow-auto p-5">
        <button onClick={onClose} className="absolute top-3 right-3 text-text-muted hover:text-text" aria-label="关闭">✕</button>
        {err && <div className="text-sev-critical p-4">{err}</div>}
        {!asset && !err && <div className="text-text-muted p-4">加载中…</div>}
        {asset && <AssetDetailPanel asset={asset} />}
      </div>
    </div>
  )
}
