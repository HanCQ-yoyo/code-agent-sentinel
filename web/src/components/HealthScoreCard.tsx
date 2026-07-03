import type { HealthScore } from '../types'
export function HealthScoreCard({ h }: { h: HealthScore | null | undefined }) {
  const score = h?.score
  return (
    <div className="bg-bg-card border border-bg-border rounded-lg p-6">
      <div className="text-sm text-slate-400">健康分</div>
      <div className={`text-5xl font-bold ${score === undefined ? 'text-slate-500' : score >= 75 ? 'text-sev-low' : score >= 60 ? 'text-sev-medium' : 'text-sev-critical'}`} data-testid="health-score-value">{score === undefined ? '--' : score}</div>
      <div className="text-slate-400">{h?.band ?? '未扫描'}</div>
    </div>
  )
}
