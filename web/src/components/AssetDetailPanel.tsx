import type { Asset } from '../types'
import { Badge, type BadgeTone } from './Badge'

// AssetDetailPanel 是资产详情的纯展示内容(抽屉与文件树右栏共用)。
export function AssetDetailPanel({ asset }: { asset: Asset }) {
  return (
    <div className="space-y-3">
      <div className="flex items-baseline gap-3 flex-wrap">
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
  )
}
