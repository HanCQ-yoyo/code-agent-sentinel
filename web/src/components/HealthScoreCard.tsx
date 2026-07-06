import type { HealthScore } from '../types'

// bandColor:健康分→环形描边色。sev-* 色为 dataviz「标记色」,
// 在浅色面上 sev-high/medium 对比度低于 AA(见 index.css 注释),
// 故仅用作环形 stroke(非文本标记),不染正文/数字。
function bandColor(score: number): string {
  if (score >= 80) return 'var(--sev-low)'
  if (score >= 60) return 'var(--sev-medium)'
  if (score >= 40) return 'var(--sev-high)'
  return 'var(--sev-critical)'
}

export function HealthScoreCard({ h }: { h?: HealthScore }) {
  const score = h?.score
  const display = score === undefined ? '--' : String(score)
  const ringColor = score === undefined ? 'var(--text-dim)' : bandColor(score)
  const pct = score === undefined ? 0 : Math.max(0, Math.min(100, score))
  const r = 52
  const c = 2 * Math.PI * r
  return (
    <div className="bg-bg-card border border-bg-border rounded-xl p-5">
      <div className="text-sm text-text-muted mb-3">健康分</div>
      <div className="flex items-center gap-5">
        <svg width="120" height="120" viewBox="0 0 120 120" aria-hidden="true">
          <circle cx="60" cy="60" r={r} fill="none" stroke="var(--bg-border)" strokeWidth="10" />
          <circle
            cx="60" cy="60" r={r} fill="none" stroke={ringColor} strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={`${(pct / 100) * c} ${c}`}
            transform="rotate(-90 60 60)"
          />
        </svg>
        <div>
          {/* CRITICAL 偏差:brief Step 4 原作 style={{color }} 染数字;
              按 Task 6 评审要求改为 text(主墨色),sev 色仅用于环形 stroke。 */}
          <div className="text-4xl font-bold text-text" data-testid="health-score-value">{display}</div>
          <div className="text-sm text-text-muted mt-1">{h?.band ?? (score === undefined ? '未扫描' : '')}</div>
        </div>
      </div>
    </div>
  )
}
