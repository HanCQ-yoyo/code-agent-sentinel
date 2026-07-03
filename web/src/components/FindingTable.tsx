import type { Finding } from '../types'

// 静态类映射:Tailwind JIT 只扫描源码里的静态完整字符串来决定生成哪些类,
// 动态拼接(如 `text-sev-${f.severity}`)的类名不会被生成 → 严重度列会无色。
// 故用静态映射替换动态拼接,视觉设计保持不变(按 severity 上色)。
const SEV_TEXT: Record<string, string> = {
  critical: 'text-sev-critical',
  high: 'text-sev-high',
  medium: 'text-sev-medium',
  low: 'text-sev-low',
}

export function FindingTable({ findings }: { findings: Finding[] }) {
  return (
    <table className="w-full text-sm">
      <thead className="text-slate-400 text-left"><tr><th className="p-2">严重度</th><th>规则</th><th>资产</th><th>说明</th><th>修复</th></tr></thead>
      <tbody>
        {findings.map((f, i) => (
          <tr key={i} className="border-t border-bg-border">
            <td className={`p-2 font-mono ${SEV_TEXT[f.severity] ?? ''}`}>{f.severity}</td>
            <td className="font-mono">{f.rule_id}</td><td>{f.asset_name}</td><td className="text-slate-400">{f.message}</td><td className="text-slate-500">{f.remediation}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
