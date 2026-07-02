export function DetectorStatusList({ list }: { list: { id: string; available: boolean; reason?: string }[] }) {
  return (
    <div className="bg-bg-card border border-bg-border rounded-lg p-4">
      <div className="text-sm text-slate-400 mb-2">检测器</div>
      {list.map(d => (
        <div key={d.id} className="flex justify-between py-1">
          <span className="font-mono">{d.id}</span>
          <span className={d.available ? 'text-sev-low' : 'text-sev-medium'}>{d.available ? 'available' : 'unavailable'}</span>
        </div>
      ))}
    </div>
  )
}
