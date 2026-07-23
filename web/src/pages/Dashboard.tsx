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

export default function Dashboard() {
  const { t } = useTranslation()
  const { dashboard, selectedAgents, fetchDashboard, history, fetchHistory, error, authError } = useStore()
  const [selectedDetector, setSelectedDetector] = useState<string | undefined>(undefined)
  useEffect(() => {
    fetchDashboard()
    fetchHistory()
    // Task 9:TEMPORARY shim — selectedAgents 替换 selectedAgent。Task 10 重建 Dashboard 聚合视图。
  }, [fetchDashboard, fetchHistory, selectedAgents])

  const detectors = dashboard?.detectors ?? []
  const counts = dashboard?.asset_counts ?? {}
  const findings = dashboard?.last_scan?.findings ?? []
  // Task 10:顶部 agent 上下文行(选中 agent 名称 + 上次扫描时间)。
  const lastScan = dashboard?.last_scan
  const agentLabel = dashboard?.agent_name ?? dashboard?.agent ?? '-'
  const lastScanTime = lastScan?.started_at ? new Date(lastScan.started_at).toLocaleString() : '-'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message={t('common.loadFailed')} description={error} showIcon /> : null}
      <div style={{ marginBottom: 12, color: 'var(--text-secondary)', fontSize: 13 }}>
        {t('dashboard.agentContext', { agent: agentLabel, time: lastScanTime })}
      </div>
      <Row gutter={16} align="stretch">
        <Col xs={24} lg={8} style={{ display: 'flex' }}><HealthScoreCard h={dashboard?.last_scan?.health_score} /></Col>
        <Col xs={24} lg={16} style={{ display: 'flex' }}><AssetStatTiles counts={counts} /></Col>
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
