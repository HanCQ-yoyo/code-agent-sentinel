import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'

export default function Findings() {
  const { scan, error } = useStore()
  return (
    <div className="space-y-4">
      {error && <div className="border border-sev-critical text-sev-critical bg-bg-card rounded-lg p-3 text-sm">扫描失败:{error}</div>}
      {!scan && <div className="text-text-muted p-8">尚未扫描 · 去看板点击"重新扫描"</div>}
      {scan && <FindingTable findings={scan.findings} />}
    </div>
  )
}
