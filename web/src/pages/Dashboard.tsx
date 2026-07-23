import { useEffect, useState } from 'react'
import { Row, Col, Alert, Typography, Card } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { DetectorPanel } from '../components/DetectorPanel'
import { AssetStatTiles } from '../components/AssetStatTiles'
import { TopRiskTypes } from '../components/TopRiskTypes'
import { RiskTrendChart } from '../components/RiskTrendChart'
import { AgentMultiSelect } from '../components/AgentMultiSelect'
import { AgentIcon } from '../components/AgentIcon'
import { formatDateTimeShort } from '../lib/format'
import type { Finding } from '../types'

export default function Dashboard() {
  const { t } = useTranslation()
  const { dashboard, selectedAgents, setSelectedAgents, fetchDashboard, history, fetchHistory, error, authError } = useStore()
  const [selectedDetector, setSelectedDetector] = useState<string | undefined>(undefined)
  useEffect(() => {
    fetchDashboard()
    fetchHistory()
  }, [fetchDashboard, fetchHistory, selectedAgents])

  const detectors = dashboard?.detectors ?? []
  const counts = dashboard?.asset_counts ?? {}
  const isAggregate = !!dashboard?.is_aggregate && Array.isArray(dashboard?.agent_scans)
  const agentScans = isAggregate ? (dashboard?.agent_scans ?? []) : []

  // Findings 来源(Task 10 决策):
  // - 聚合模式:拼接所有 agent_scans[].last_scan?.findings,每条 finding 自带 agent_id(Task 2),
  //   SeverityChart/TopRiskTypes 显示合并视图(不做跨 agent 健康分聚合 —— 每个圆圈独立评分)。
  // - 单 agent 模式:沿用 dashboard.last_scan.findings。
  const findings: Finding[] = isAggregate
    ? agentScans.flatMap((as) => as.last_scan?.findings ?? [])
    : (dashboard?.last_scan?.findings ?? [])

  // 健康分圆圈:聚合模式每 agent 一张;单 agent 模式一张。
  // 每张圆圈外附 agent 名标签,让用户区分哪个圆圈对应哪个 agent。
  const healthCards = isAggregate
    ? agentScans.map((as) => ({
        key: as.agent_id,
        agentId: as.agent_id,
        agentName: as.agent_name,
        h: as.last_scan?.health_score,
        lastScanAt: as.last_scan?.started_at,
      }))
    : [{
        key: dashboard?.agent ?? 'single',
        agentId: dashboard?.agent ?? '',
        agentName: dashboard?.agent_name ?? dashboard?.agent ?? '-',
        h: dashboard?.last_scan?.health_score,
        lastScanAt: dashboard?.last_scan?.started_at,
      }]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      {/* 顶部:多 agent 筛选器 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <Typography.Text strong>{t('dashboard.multiAgentView')}</Typography.Text>
        <AgentMultiSelect value={selectedAgents} onChange={setSelectedAgents} />
      </div>
      {/* 健康分圆圈行:聚合模式多圆圈,单 agent 模式单圆圈。每圆圈附 agent 名 + 上次扫描时间。 */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'stretch' }}>
        {healthCards.map((c) => (
          <div key={c.key} style={{ flex: '1 1 220px', minWidth: 220, maxWidth: 320, display: 'flex', flexDirection: 'column', gap: 4 }}>
            <HealthScoreCard h={c.h} />
            <div style={{ textAlign: 'center', color: 'var(--text-secondary)', fontSize: 12 }}>
              <AgentIcon id={c.agentId} /> {c.agentName}
              {c.lastScanAt ? ` · ${formatDateTimeShort(c.lastScanAt)}` : ` · ${t('dashboard.notScanned')}`}
            </div>
          </div>
        ))}
      </div>
      <Row gutter={16} align="stretch">
        <Col xs={24} lg={24} style={{ display: 'flex' }}><AssetStatTiles counts={counts} /></Col>
      </Row>
      <Card title={t('dashboard.detectorStatus')} size="small">
        <DetectorPanel detectors={detectors} selectedId={selectedDetector} onSelect={setSelectedDetector} />
      </Card>
      <Row gutter={16}>
        <Col xs={24} lg={12}><SeverityChart findings={findings} /></Col>
        <Col xs={24} lg={12}><TopRiskTypes findings={findings} /></Col>
      </Row>
      <RiskTrendChart history={history} />
      {authError ? <Typography.Text type="warning">{t('dashboard.tokenExpired')}</Typography.Text> : null}
    </div>
  )
}
