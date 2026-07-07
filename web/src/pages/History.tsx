import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Table, Button, Card, Row, Col, Spin, Empty, Typography } from 'antd'
import { ArrowLeftOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useStore } from '../store'
import type { ScanSummary, ScanRecord } from '../types'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { FindingTable } from '../components/FindingTable'

export default function History() {
  const { id } = useParams<{ id: string }>()
  const { history, fetchHistory, fetchHistoryDetail, deleteHistory } = useStore()
  const [detail, setDetail] = useState<ScanRecord | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!id) { fetchHistory(); return }
    setDetail(null); setErr('')
    fetchHistoryDetail(id).then((r) => { if (r) setDetail(r); else setErr('加载失败') }).catch((e) => setErr(String(e)))
  }, [id, fetchHistory, fetchHistoryDetail])

  if (id) {
    if (err) return <Card>{err}</Card>
    if (!detail) return <Spin style={{ display: 'block', margin: '40px auto' }} />
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Link to="/history"><Button type="link" icon={<ArrowLeftOutlined />}>返回历史列表</Button></Link>
        <Typography.Title level={5} style={{ color: 'var(--text)' }}>{detail.id} · {detail.started_at}</Typography.Title>
        <Row gutter={16}>
          <Col xs={24} lg={12}><HealthScoreCard h={detail.health_score} /></Col>
          <Col xs={24} lg={12}><SeverityChart findings={detail.findings ?? []} /></Col>
        </Row>
        <FindingTable findings={detail.findings ?? []} />
      </div>
    )
  }

  const columns: ColumnsType<ScanSummary> = [
    { title: '时间', dataIndex: 'started_at', render: (t: string, h: ScanSummary) => <Link to={`/history/${h.id}`}><span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{t}</span></Link> },
    { title: '健康分', width: 120, render: (_: unknown, h: ScanSummary) => <span style={{ fontFamily: 'var(--font-mono)' }}>{h.health_score} · {h.band}</span> },
    { title: '发现', dataIndex: 'finding_count', width: 80 },
    { title: '检测器', width: 120, render: (_: unknown, h: ScanSummary) => <span style={{ fontFamily: 'var(--font-mono)' }}>{h.detector_avail}/{h.detector_total}</span> },
    { title: '', width: 80, render: (_: unknown, h: ScanSummary) => <Button danger size="small" icon={<DeleteOutlined />} onClick={() => deleteHistory(h.id)} /> },
  ]

  return (
    <Card>
      {history.length === 0 ? <Empty description="暂无历史扫描" /> : (
        <Table<ScanSummary> rowKey="id" columns={columns} dataSource={history} pagination={false} size="middle" />
      )}
    </Card>
  )
}
