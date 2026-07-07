// web/src/pages/History.tsx
import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { FindingTable } from '../components/FindingTable'
import type { ScanRecord } from '../types'

export default function History() {
  const { id } = useParams<{ id: string }>()
  const { history, fetchHistory, fetchHistoryDetail, deleteHistory } = useStore()
  const [detail, setDetail] = useState<ScanRecord | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => { fetchHistory() }, [fetchHistory])

  useEffect(() => {
    if (!id) { setDetail(null); return }
    setDetail(null); setErr('')
    fetchHistoryDetail(id).then(r => { if (r) setDetail(r); else setErr('加载失败') }).catch(e => setErr(String(e)))
  }, [id, fetchHistoryDetail])

  // 详情视图:复用看板组件,传历史数据
  if (id) {
    if (err) return <div className="text-text p-8 border-l-2 border-sev-critical">{err}</div>
    if (!detail) return <div className="text-text-muted p-8">加载中…</div>
    return (
      <div className="space-y-4">
        <Link to="/history" className="text-sm text-accent">← 返回历史列表</Link>
        <div className="text-sm text-text-muted font-mono">{detail.id} · {detail.started_at}</div>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <HealthScoreCard h={detail.health_score} />
          <SeverityChart findings={detail.findings ?? []} />
        </div>
        <FindingTable findings={detail.findings ?? []} />
      </div>
    )
  }

  // 列表视图
  return (
    <div className="space-y-4">
      <div className="text-sm text-text-muted">历史扫描记录</div>
      {!history || history.length === 0 ? (
        <div className="text-text-muted p-8 text-center">暂无历史扫描</div>
      ) : (
        <div className="bg-bg-card border border-bg-border rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead className="text-text-muted text-left border-b border-bg-border">
              <tr><th className="p-2">时间</th><th className="p-2 w-24">健康分</th><th className="p-2 w-20">发现</th><th className="p-2 w-32">检测器</th><th className="p-2 w-16"></th></tr>
            </thead>
            <tbody>
              {history.map(h => (
                <tr key={h.id} className="border-b border-bg-border/50">
                  <td className="p-2"><Link to={`/history/${h.id}`} className="text-accent font-mono text-xs">{h.started_at}</Link></td>
                  <td className="p-2">{h.health_score} · {h.band}</td>
                  <td className="p-2">{h.finding_count}</td>
                  <td className="p-2 text-text-muted">{h.detector_avail}/{h.detector_total}</td>
                  <td className="p-2">
                    <button onClick={() => deleteHistory(h.id)} className="text-text-muted hover:text-text text-xs hover:underline">删除</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
