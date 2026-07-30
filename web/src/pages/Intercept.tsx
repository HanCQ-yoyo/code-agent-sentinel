import { useEffect, useState } from 'react'
import { Card, Table, Tag, Drawer, Select, Button, Space, Tooltip, Empty, Spin, Popconfirm, Typography, message } from 'antd'
import { ReloadOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { InterceptRecord } from '../types'
import { formatDateTime } from '../lib/format'

// outcome → Tag color(deny=红 / warn=橙 / ask=金 / allow=默认灰)。
// 与 FindingTable/History 风险色阶方向一致:越严重越红。
const outcomeColor: Record<string, string> = {
  deny: 'red',
  warn: 'orange',
  ask: 'gold',
  allow: 'default',
}

// confidence → Tag color(Stage R3)。high=绿(明确命中)/ low=橙(降级)/ unknown=默认灰。
// 仅 deny/warn 且规则引擎填充时才有值;allow 记录缺省。
const confidenceColor: Record<string, string> = {
  high: 'green',
  low: 'orange',
  unknown: 'default',
}

export default function Intercept() {
  const { t } = useTranslation()
  const { intercept, fetchIntercepts, fetchInterceptDetail, deleteIntercept } = useStore()
  const [outcome, setOutcome] = useState<string>('')
  const [detail, setDetail] = useState<InterceptRecord | null>(null)
  const [open, setOpen] = useState(false)
  const [loadingDetail, setLoadingDetail] = useState(false)

  // 初次挂载 + outcome 筛选变化时拉列表。outcome 空=不过滤(全部)。
  useEffect(() => {
    fetchIntercepts(outcome || undefined)
  }, [fetchIntercepts, outcome])

  // 点行 → 拉详情 → 开抽屉。拉取失败(message.error 提示,不崩)。
  const openDetail = async (id: string) => {
    setLoadingDetail(true)
    try {
      const r = await fetchInterceptDetail(id)
      if (r) {
        setDetail(r)
        setOpen(true)
      } else {
        // wrap 返回 undefined(authError 或网络错);store 已 set error,此处补 toast。
        message.error(t('common.loadFailed'))
      }
    } finally {
      setLoadingDetail(false)
    }
  }

  // 删除单条:成功后关抽屉 + 重拉列表(store.deleteIntercept 内部已 fetchIntercepts)。
  const onDelete = async (id: string) => {
    await deleteIntercept(id)
    setOpen(false)
    setDetail(null)
  }

  const columns: ColumnsType<InterceptRecord> = [
    {
      title: t('intercept.time'),
      dataIndex: 'timestamp',
      width: 170,
      render: (v: string) => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{formatDateTime(v)}</span>
      ),
    },
    {
      title: t('intercept.outcome'),
      dataIndex: 'outcome',
      width: 90,
      render: (v: string) => <Tag color={outcomeColor[v] ?? 'default'}>{v}</Tag>,
    },
    {
      title: t('intercept.command'),
      dataIndex: 'command',
      ellipsis: { showTitle: false },
      render: (v: string) => (
        <Tooltip title={v}><Typography.Text code style={{ fontSize: 12 }}>{v}</Typography.Text></Tooltip>
      ),
    },
    {
      title: t('intercept.rule'),
      dataIndex: 'rule_id',
      width: 160,
      ellipsis: true,
      render: (v?: string) => v ? (
        <Typography.Text code style={{ fontSize: 12 }}>{v}</Typography.Text>
      ) : <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: t('intercept.severity'),
      dataIndex: 'severity',
      width: 90,
      render: (v?: string) => v ? <Tag>{v}</Tag> : <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: t('intercept.confidence'),
      dataIndex: 'confidence',
      width: 110,
      render: (v?: string) => v ? (
        <Tag color={confidenceColor[v] ?? 'default'}>{v}</Tag>
      ) : <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: t('intercept.duration'),
      dataIndex: 'eval_duration_us',
      width: 100,
      render: (v: number) => <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{v}μs</span>,
    },
  ]

  return (
    <Card>
      {/* 筛选作为表的控制头(filter-toolbar 统一模式:控制层/数据层同卡内分层)。 */}
      <div className="filter-toolbar">
        <Select
          allowClear
          placeholder={t('intercept.filterOutcome')}
          style={{ width: 180 }}
          value={outcome || undefined}
          options={['deny', 'warn', 'ask', 'allow'].map((o) => ({ label: o, value: o }))}
          onChange={(v) => setOutcome(v ?? '')}
        />
        <Button icon={<ReloadOutlined />} onClick={() => fetchIntercepts(outcome || undefined)}>
          {t('intercept.refresh')}
        </Button>
      </div>
      <Table<InterceptRecord>
        rowKey="id"
        columns={columns}
        dataSource={intercept}
        size="small"
        pagination={{ defaultPageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'], showTotal: (total) => t('history.totalCount', { count: total }), size: 'small' }}
        onRow={(r) => ({ onClick: () => openDetail(r.id), style: { cursor: 'pointer' } })}
        locale={{ emptyText: <Empty description={t('intercept.empty')} /> }}
      />
      <Drawer
        open={open}
        onClose={() => setOpen(false)}
        width={560}
        title={t('intercept.detail')}
        destroyOnClose
      >
        {loadingDetail ? <Spin style={{ display: 'block', margin: '40px auto' }} /> : detail && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.command')}</Typography.Text>
              <pre style={{ background: 'var(--surface-2)', padding: 8, borderRadius: 4, fontSize: 12, fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: '4px 0 0' }}>
                {detail.command}
              </pre>
            </div>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.outcome')}</Typography.Text>
              <div style={{ marginTop: 4 }}><Tag color={outcomeColor[detail.outcome] ?? 'default'}>{detail.outcome}</Tag></div>
            </div>
            {detail.reason && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.reason')}</Typography.Text>
                <Typography.Paragraph style={{ margin: '4px 0 0' }}>{detail.reason}</Typography.Paragraph>
              </div>
            )}
            {detail.rule_id && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.rule')}</Typography.Text>
                <div style={{ marginTop: 4 }}><Typography.Text code>{detail.rule_id}</Typography.Text></div>
              </div>
            )}
            {detail.severity && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.severity')}</Typography.Text>
                <div style={{ marginTop: 4 }}><Tag>{detail.severity}</Tag></div>
              </div>
            )}
            {detail.confidence && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.confidence')}</Typography.Text>
                <div style={{ marginTop: 4 }}>
                  <Tag color={confidenceColor[detail.confidence] ?? 'default'}>{detail.confidence}</Tag>
                </div>
              </div>
            )}
            {detail.matched_span && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.matchedSpan')}</Typography.Text>
                <pre style={{ background: 'var(--surface-2)', padding: 8, borderRadius: 4, fontSize: 12, fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: '4px 0 0' }}>
                  {detail.matched_span}
                </pre>
              </div>
            )}
            {detail.working_dir && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.cwd')}</Typography.Text>
                <div style={{ marginTop: 4 }}><Typography.Text code style={{ fontSize: 11 }}>{detail.working_dir}</Typography.Text></div>
              </div>
            )}
            {detail.session_id && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.session')}</Typography.Text>
                <div style={{ marginTop: 4 }}><Typography.Text code style={{ fontSize: 11 }}>{detail.session_id}</Typography.Text></div>
              </div>
            )}
            {detail.tool_name && (
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.tool')}</Typography.Text>
                <div style={{ marginTop: 4 }}><Typography.Text code>{detail.tool_name}</Typography.Text></div>
              </div>
            )}
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t('intercept.duration')}</Typography.Text>
              <div style={{ marginTop: 4, fontFamily: 'var(--font-mono)', fontSize: 12 }}>{detail.eval_duration_us}μs</div>
            </div>
            <Popconfirm
              title={t('intercept.confirmDelete')}
              okText={t('intercept.delete')}
              okButtonProps={{ danger: true }}
              cancelText={t('common.cancel')}
              onConfirm={() => onDelete(detail.id)}
            >
              <Button danger icon={<DeleteOutlined />} style={{ alignSelf: 'flex-start', marginTop: 8 }}>
                {t('intercept.delete')}
              </Button>
            </Popconfirm>
          </div>
        )}
      </Drawer>
    </Card>
  )
}
