import { useStore } from '../store'
import { FindingTable } from '../components/FindingTable'
export default function Findings() {
  const scan = useStore(s => s.scan)
  return <div className="bg-bg-card border border-bg-border rounded-lg p-2"><FindingTable findings={scan?.findings ?? []} /></div>
}
