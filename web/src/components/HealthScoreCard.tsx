import type { HealthScore } from '../types'
export function HealthScoreCard({ h }: { h: HealthScore | null | undefined }) {
  const score = h?.score ?? 100
  return (
    <div className="bg-bg-card border border-bg-border rounded-lg p-6">
      <div className="text-sm text-slate-400">健康分</div>
      <div className={`text-5xl font-bold ${score >= 75 ? 'text-sev-low' : score >= 60 ? 'text-sev-medium' : 'text-sev-critical'}`}>{score}</div>
      <div className="text-slate-400">{h?.band ?? 'Excellent'}</div>
    </div>
  )
}
