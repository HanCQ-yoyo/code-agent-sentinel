import type { Asset } from '../types'
export function AssetList({ assets }: { assets: Asset[] }) {
  return (
    <table className="w-full text-sm">
      <thead className="text-slate-400 text-left"><tr><th className="p-2">类型</th><th>名称</th><th>scope</th><th>路径</th></tr></thead>
      <tbody>
        {assets.map(a => (
          <tr key={a.id} className="border-t border-bg-border">
            <td className="p-2 font-mono text-sev-low">{a.type}</td>
            <td>{a.name}</td><td>{a.scope}</td><td className="text-slate-500 truncate max-w-xs">{a.source_path}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
