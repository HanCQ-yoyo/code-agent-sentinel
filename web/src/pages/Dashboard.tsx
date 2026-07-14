import { useEffect, useState } from 'react'
import { Row, Col, Alert, Typography, Card } from 'antd'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { DetectorPanel } from '../components/DetectorPanel'
import { AssetStatTiles } from '../components/AssetStatTiles'
import { TopRiskTypes } from '../components/TopRiskTypes'
import { RiskTrendChart } from '../components/RiskTrendChart'

export default function Dashboard() {
  const { dashboard, fetchDashboard, history, fetchHistory, error, authError } = useStore()
  const [selectedDetector, setSelectedDetector] = useState<string | undefined>(undefined)
  useEffect(() => {
    fetchDashboard()
    fetchHistory()
  }, [fetchDashboard, fetchHistory])

  const detectors = dashboard?.detectors ?? []
  const counts = dashboard?.asset_counts ?? {}
  const findings = dashboard?.last_scan?.findings ?? []

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message="加载失败" description={error} showIcon /> : null}
      <Row gutter={16}>
        <Col xs={24} lg={8}><HealthScoreCard h={dashboard?.last_scan?.health_score} /></Col>
        <Col xs={24} lg={16}><AssetStatTiles counts={counts} /></Col>
      </Row>
      <Card title="检测器状态" size="small">
        <DetectorPanel detectors={detectors} selectedId={selectedDetector} onSelect={setSelectedDetector} />
      </Card>
      <Row gutter={16}>
        <Col xs={24} lg={12}><SeverityChart findings={findings} /></Col>
        <Col xs={24} lg={12}><TopRiskTypes findings={findings} /></Col>
      </Row>
      <RiskTrendChart history={history} />
      {authError ? <Typography.Text type="warning">token 失效,请重新带 #token= 访问。</Typography.Text> : null}
    </div>
  )
}
