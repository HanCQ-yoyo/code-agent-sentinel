import { useEffect } from 'react'
import { useStore } from '../store'
import { DetectorStatusList } from '../components/DetectorStatus'

export default function Settings() {
  const { detectors, fetchDetectors } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div className="space-y-4 max-w-3xl">
      <div className="bg-bg-card border border-bg-border rounded-xl p-5">
        <h2 className="text-lg font-semibold mb-1">设置(只读)</h2>
        <p className="text-sm text-text-muted mb-4">P1 阶段所有配置资产只读。检测器状态如下;缺失的子进程检测器会优雅降级,不阻断扫描。</p>
        <DetectorStatusList list={detectors} bare />
      </div>
      <div className="bg-bg-card border border-bg-border rounded-xl p-5">
        <h2 className="text-lg font-semibold mb-1">关于</h2>
        <div className="text-sm text-text-muted space-y-1">
          <div>规则版本:P1 内置基线 / 提示注入规则集(embedded)</div>
          <div>密钥检测:依赖 gitleaks(缺失时跳过),P2 将重心转移到 MCP/Skills/Scripts 定向检测</div>
        </div>
      </div>
    </div>
  )
}
