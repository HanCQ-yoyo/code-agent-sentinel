import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Table, Button, Card, Row, Col, Spin, Empty, Typography, Alert, Popconfirm, Select, Switch, Tag, Collapse } from 'antd'
import { ArrowLeftOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { ScanSummary, ScanRecord } from '../types'
import { HealthScoreCard } from '../components/HealthScoreCard'
import { SeverityChart } from '../components/SeverityChart'
import { FindingTable } from '../components/FindingTable'
import { formatDateTime, formatDateTimeShort } from '../lib/format'
import { agentMetaById } from '../lib/agents'

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
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const { history, agents, fetchHistory, fetchHistoryDetail, deleteHistory } = useStore()
  const [detail, setDetail] = useState<ScanRecord | null>(null)
  const [err, setErr] = useState('')
  // Task 10:客户端 agent 筛选(历史列表本身是全局的,展示所有 agent 的扫描;下拉仅本地过滤)。
  // 必须在 early return(detail 视图)之前调用:React Router 在 /history 与 /history/:id 间复用
  // 同一 <History /> 实例(无 remount key),若 useState 在 early return 之后,列表视图与详情视图
  // 的 hook 数量不一致 → "Rendered fewer hooks than expected" → 页面崩溃(白屏)。
  const [agentFilter, setAgentFilter] = useState<string>('')
  // Task 12:按批次分组开关(列表视图)。默认关闭,保持原平铺 Table。
  const [batchGroup, setBatchGroup] = useState(false)
  // Task 12:batch 详情视图 — 同批次各 agent 的 ScanRecord[](detail 是主入口,
  // siblings 是同 batch 其他 agent 的记录;若 detail.batch_id 为空 → siblings 为空,走单记录视图)。
  const [batchSiblings, setBatchSiblings] = useState<ScanRecord[]>([])
  const [batchLoading, setBatchLoading] = useState(false)

  // Task 12:fetchHistory 始终调用(列表 + 详情视图都需要 history —— 详情视图靠它找 batch sibling)。
  // 原设计仅 !id 时 fetchHistory;Task 12 详情页需 history 按 batch_id 查 sibling,故无条件拉取。
  useEffect(() => {
    fetchHistory()
  }, [fetchHistory])

  // 主记录拉取:id 变化(进入/离开详情)时触发。
  useEffect(() => {
    if (!id) { setDetail(null); setErr(''); setBatchSiblings([]); return }
    setDetail(null); setErr('')
    setBatchSiblings([]); setBatchLoading(false)
    fetchHistoryDetail(id).then((r) => {
      if (!r) { setErr(t('common.loadFailed')); return }
      setDetail(r)
    }).catch((e) => setErr(String(e)))
  }, [id, fetchHistoryDetail, t])

  // Task 12:batch sibling 拉取 — 在 detail 和 history 都就绪后触发。
  // 若 detail.batch_id 非空,从 history 找同 batch 的 sibling id,逐条 fetchHistoryDetail。
  // 单条记录(batch_id 空)或 history 未就绪 → 跳过(保持单记录视图)。
  useEffect(() => {
    if (!detail?.batch_id || history.length === 0) { setBatchSiblings([]); return }
    const sibIds = history.filter((h) => h.batch_id === detail.batch_id && h.id !== detail.id).map((h) => h.id)
    if (sibIds.length === 0) { setBatchSiblings([]); return }
    let cancelled = false
    setBatchLoading(true)
    Promise.all(sibIds.map((sid) => fetchHistoryDetail(sid).catch(() => undefined)))
      .then((results) => {
        if (cancelled) return
        setBatchSiblings(results.filter((x): x is ScanRecord => !!x))
      })
      .finally(() => { if (!cancelled) setBatchLoading(false) })
    return () => { cancelled = true }
  }, [detail, history, fetchHistoryDetail])

  if (id) {
    if (err) return <Alert type="error" message={t('common.loadFailed')} description={err} showIcon style={{ margin: 16 }} />
    if (!detail) return <Spin style={{ display: 'block', margin: '40px auto' }} />
    // Task 12:batch 详情视图 — 同批次所有 agent 的记录合并展示。
    // batchRecords = [detail, ...batchSiblings](主入口 + siblings);单记录(batch_id 空) → 只 detail。
    // 健康分:每 agent 独立 HealthScoreCard(绝不跨 agent 聚合分数,遵循健康分公式硬原则)。
    // Findings:合并所有 sibling 的 findings(每条已带 agent_id),复用 FindingTable(含 Agent 列)。
    const batchRecords: ScanRecord[] = batchSiblings.length > 0 ? [detail, ...batchSiblings] : [detail]
    const isBatch = !!detail.batch_id && batchSiblings.length > 0
    const mergedFindings = batchRecords.flatMap((r) => r.findings ?? [])
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Link to="/history"><Button type="link" icon={<ArrowLeftOutlined />}>{t('history.backToList')}</Button></Link>
        <Typography.Title level={5} style={{ color: 'var(--text)', fontFamily: 'var(--font-mono)' }}>
          {detail.id} · {formatDateTime(detail.started_at)}
          {isBatch ? <Tag style={{ marginLeft: 8 }} color="blue">{t('history.batchTag', { id: detail.batch_id!.slice(-8) })}</Tag> : null}
        </Typography.Title>
        {batchLoading ? <Spin size="small" style={{ display: 'block', margin: '8px auto' }} /> : null}
        {/* 健康分圆圈行:batch 模式每 agent 一张;单记录模式一张。每张附 agent 名。 */}
        <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'stretch' }}>
          {batchRecords.map((r) => {
            const aid = r.agent_id ?? ''
            const m = agentMetaById(aid)
            return (
              <div key={r.id} style={{ flex: '1 1 220px', minWidth: 220, maxWidth: 320, display: 'flex', flexDirection: 'column', gap: 4 }}>
                <HealthScoreCard h={r.health_score} />
                <div style={{ textAlign: 'center', color: 'var(--text-secondary)', fontSize: 12 }}>
                  {m.icon} {m.label}
                </div>
              </div>
            )
          })}
        </div>
        {/* SeverityChart:合并所有 batchRecords 的 findings(单记录 = detail.findings)。
            batch 模式右侧补一张信息卡说明 agent 数(避免左 SeverityChart 单独占半行)。 */}
        <Row gutter={16} align="stretch">
          <Col xs={24} lg={12} style={{ display: 'flex' }}><SeverityChart findings={mergedFindings} /></Col>
          <Col xs={24} lg={12} style={{ display: 'flex' }}>
            <Card style={{ width: '100%' }} size="small">
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {isBatch
                  ? t('history.batchAgentsCount', { count: batchRecords.length })
                  : t('history.singleAgentHint')}
              </Typography.Text>
            </Card>
          </Col>
        </Row>
        <FindingTable findings={mergedFindings} startedAt={detail.started_at} />
      </div>
    )
  }

  const columns: ColumnsType<ScanSummary> = [
    { title: t('history.colTime'), dataIndex: 'started_at', width: 150, render: (time: string, h: ScanSummary) => <Link to={`/history/${h.id}`}><span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{formatDateTimeShort(time)}</span></Link> },
    { title: t('history.agent'), dataIndex: 'agent_id', width: 120, render: (id: string) => { const m = agentMetaById(id ?? ''); return id ? <span style={{ whiteSpace: 'nowrap' }}>{m.icon} {m.label}</span> : '-' } },
    { title: t('history.colRiskScore'), width: 90, render: (_: unknown, h: ScanSummary) => (
      <span title={h.band} style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: riskColor(h.health_score) }}>{h.health_score}</span>
    ) },
    // Task 12:列名由「发现」(colFindings)改为「风险数量」(colRiskCount),语义更准确。
    { title: t('history.colRiskCount'), dataIndex: 'finding_count', width: 90 },
    { title: t('history.colDetectors'), width: 120, render: (_: unknown, h: ScanSummary) => <span style={{ fontFamily: 'var(--font-mono)' }}>{h.detector_avail}/{h.detector_total}</span> },
    // Task 12:Batch 列 — 显示 batch_id 末 8 位(同次重扫共享);无 batch_id(旧/单 agent 扫描)显示 '-'。
    { title: t('history.colBatch'), dataIndex: 'batch_id', width: 110, render: (bid?: string) => bid ? <Tag color="blue" style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{bid.slice(-8)}</Tag> : '-' },
    { title: t('history.colAction'), width: 80, render: (_: unknown, h: ScanSummary) => (
      <Popconfirm title={t('history.confirmDelete')} okText={t('history.delete')} okButtonProps={{ danger: true }} cancelText={t('common.cancel')} onConfirm={() => deleteHistory(h.id)}>
        <Button danger size="small" icon={<DeleteOutlined />} aria-label={t('history.delete')} />
      </Popconfirm>
    ) },
  ]

  // Task 10:filteredHistory 仅列表视图使用,放在 early return 之后即可(非 hook)。
  const filteredHistory = agentFilter ? history.filter(h => h.agent_id === agentFilter) : history

  // Task 12:按 batch_id 分组(仅分组模式开启时用)。
  // batch_id 非空的记录按 batch 归组;batch_id 空的记录各自成组(单条展示,保持可见)。
  // 用 Map 保持插入序(同 batch 的记录相邻)。key = batch_id || `__single_${h.id}`(保证唯一)。
  const batchGroups: { key: string; batchId?: string; rows: ScanSummary[] }[] = []
  if (batchGroup) {
    const map = new Map<string, ScanSummary[]>()
    for (const h of filteredHistory) {
      const k = h.batch_id || `__single_${h.id}`
      ;(map.get(k) ?? map.set(k, []).get(k)!)!.push(h)
    }
    for (const [k, rows] of map) {
      const bid = rows[0].batch_id
      batchGroups.push({ key: k, batchId: bid, rows })
    }
  }

  return (
    <Card>
      {/* Task 10:agent 筛选下拉 + Task 12:按批次分组开关。 */}
      <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <Select
          allowClear
          placeholder={t('history.filterAgent')}
          style={{ width: 180 }}
          value={agentFilter || undefined}
          options={(agents?.agents ?? []).map(a => ({ value: a.id, label: a.name }))}
          onChange={(v) => setAgentFilter(v ?? '')}
        />
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, color: 'var(--text-secondary)', fontSize: 12 }}>
          {t('history.batchGroup')}
          <Switch size="small" checked={batchGroup} onChange={setBatchGroup} />
        </span>
      </div>
      {filteredHistory.length === 0 ? <Empty description={(!agentFilter || history.length === 0) ? t('history.empty') : t('history.noMatchAgent')} /> : batchGroup ? (
        // 分组模式:每个 batch 一个 Collapse.Panel,标题 = 批次末8位 · N 个 Agent · 最早时间。
        // 无 batch_id 的单条记录各自成组,标题显示 scan id(不混入 batch 语义)。
        <Collapse accordion={false}>
          {batchGroups.map((g) => {
            const agentCount = new Set(g.rows.map((r) => r.agent_id)).size
            const t0 = g.rows.map((r) => r.started_at).sort()[0]
            const header = g.batchId
              ? `${t('history.batchTag', { id: g.batchId.slice(-8) })} · ${t('history.batchNAgents', { count: agentCount })} · ${formatDateTimeShort(t0)}`
              : `${g.rows[0].id} · ${formatDateTimeShort(t0)}`
            return (
              <Collapse.Panel key={g.key} header={header}>
                <Table<ScanSummary>
                  rowKey="id"
                  columns={columns.filter((c) => ('dataIndex' in c ? c.dataIndex !== 'batch_id' : true))}
                  dataSource={g.rows}
                  pagination={false}
                  size="small"
                />
              </Collapse.Panel>
            )
          })}
        </Collapse>
      ) : (
        <Table<ScanSummary>
          rowKey="id"
          columns={columns}
          dataSource={filteredHistory}
          // defaultPageSize(非受控)而非 pageSize(受控):避免页大小选择器改动被受控 pageSize
          // 重置回 20(详见 AssetTable 注释)。
          pagination={{ defaultPageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], showTotal: (total) => t('history.totalCount', { count: total }), size: 'small' }}
          size="middle"
        />
      )}
    </Card>
  )
}
