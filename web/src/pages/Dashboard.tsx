import { useEffect } from 'react'
import { Row, Col, Alert, Typography } from 'antd'
import { useStore } from '../store'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { DetectorStatusList } from '../components/DetectorStatus'

export default function Dashboard() {
  const { scan, detectors, fetchDetectors, error, authError } = useStore()
  useEffect(() => { fetchDetectors() }, [fetchDetectors])
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error ? <Alert type="error" message="加载失败" description={error} showIcon /> : null}
      <Row gutter={16}>
        <Col xs={24} lg={8}><HealthScoreCard h={scan?.health_score} /></Col>
        <Col xs={24} lg={8}><SeverityChart findings={scan?.findings ?? []} /></Col>
        <Col xs={24} lg={8}><DetectorStatusList list={detectors} /></Col>
      </Row>
      {authError ? <Typography.Text type="warning">token 失效,请重新带 #token= 访问。</Typography.Text> : null}
    </div>
  )
}
