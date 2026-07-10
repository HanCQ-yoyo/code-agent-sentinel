import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Table, Button, Card, Row, Col, Spin, Empty, Typography, Alert, Popconfirm } from 'antd'
import { ArrowLeftOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useStore } from '../store'
import type { ScanSummary, ScanRecord } from '../types'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { FindingTable } from '../components/FindingTable'
import { formatDateTime, formatDateTimeShort } from '../lib/format'

// 风险指数色:score → sev token(复用现有 4 级绿→红色阶,与 band 5 级对齐)。
// Excellent(≥90)/Good(≥75)同属健康,合 sev-low(绿);Fair→medium;At-Risk→high;Critical→critical。
// 与 HealthScoreCard.bandColor 阈值方向一致(sev 色作数字标记)。文本穿 text 色,色仅着数字。
function riskColor(score: number): string {
  if (score >= 90) return 'var(--sev-low)'
  if (score >= 75) return 'var(--sev-low)'
  if (score >= 60) return 'var(--sev-medium)'
  if (score >= 40) return 'var(--sev-high)'
  return 'var(--sev-critical)'
}

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
    if (err) return <Alert type="error" message="加载失败" description={err} showIcon style={{ margin: 16 }} />
    if (!detail) return <Spin style={{ display: 'block', margin: '40px auto' }} />
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Link to="/history"><Button type="link" icon={<ArrowLeftOutlined />}>返回历史列表</Button></Link>
        <Typography.Title level={5} style={{ color: 'var(--text)', fontFamily: 'var(--font-mono)' }}>
          {detail.id} · {formatDateTime(detail.started_at)}
        </Typography.Title>
        <Row gutter={16}>
          <Col xs={24} lg={12}><HealthScoreCard h={detail.health_score} /></Col>
          <Col xs={24} lg={12}><SeverityChart findings={detail.findings ?? []} /></Col>
        </Row>
        <FindingTable findings={detail.findings ?? []} />
      </div>
    )
  }

  const columns: ColumnsType<ScanSummary> = [
    { title: '时间', dataIndex: 'started_at', width: 150, render: (t: string, h: ScanSummary) => <Link to={`/history/${h.id}`}><span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{formatDateTimeShort(t)}</span></Link> },
    { title: '风险指数', width: 90, render: (_: unknown, h: ScanSummary) => (
      <span title={h.band} style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: riskColor(h.health_score) }}>{h.health_score}</span>
    ) },
    { title: '发现', dataIndex: 'finding_count', width: 80 },
    { title: '检测器', width: 120, render: (_: unknown, h: ScanSummary) => <span style={{ fontFamily: 'var(--font-mono)' }}>{h.detector_avail}/{h.detector_total}</span> },
    { title: '操作', width: 80, render: (_: unknown, h: ScanSummary) => (
      <Popconfirm title="确认删除此扫描记录?" okText="删除" okButtonProps={{ danger: true }} cancelText="取消" onConfirm={() => deleteHistory(h.id)}>
        <Button danger size="small" icon={<DeleteOutlined />} aria-label="删除" />
      </Popconfirm>
    ) },
  ]

  return (
    <Card>
      {history.length === 0 ? <Empty description="暂无历史扫描" /> : (
        <Table<ScanSummary> rowKey="id" columns={columns} dataSource={history} pagination={false} size="middle" />
      )}
    </Card>
  )
}
