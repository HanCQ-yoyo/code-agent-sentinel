import { useEffect, useState } from 'react'
import { Card, Table, Tag, Drawer, Select, Button, Space, Tooltip, Empty, Spin, Popconfirm, Typography, message, Modal, Input, Form } from 'antd'
import { ReloadOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { InterceptRecord, RuleDTO } from '../types'
import { formatDateTime } from '../lib/format'

// 转义正则元字符:拦截名单按精确命令匹配,但新建规则用 regex,需转义命令中的特殊字符。
function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// outcome → 实色背景 token(deny=红 / warn=橙 / ask=金 / allow=默认灰)。
// 与 FindingTable/History 风险色阶方向一致:越严重越红。
const outcomeColor: Record<string, string> = {
  deny: 'var(--sev-critical-solid)',
  warn: 'var(--sev-high-solid)',
  ask: 'var(--cat-3)',
  allow: 'var(--color-rule-2)',
}

// confidence → 实色背景 token(Stage R3)。high=绿(明确命中)/ low=橙(降级)/ unknown=默认灰。
// 仅 deny/warn 且规则引擎填充时才有值;allow 记录缺省。
const confidenceColor: Record<string, string> = {
  high: 'var(--sev-low-solid)',
  low: 'var(--sev-high-solid)',
  unknown: 'var(--color-rule-2)',
}

export default function Intercept() {
  const { t } = useTranslation()
  const { intercept, fetchIntercepts, fetchInterceptDetail, deleteIntercept, saveRule, allowlist, saveAllowlist } = useStore()
  const [outcome, setOutcome] = useState<string>('')
  const [detail, setDetail] = useState<InterceptRecord | null>(null)
  const [open, setOpen] = useState(false)
  const [loadingDetail, setLoadingDetail] = useState(false)
  // 处置弹框:disposeRec=处置中的记录(null=弹框关闭);disposeCmd=可编辑命令(初始=记录.command)。
  const [disposeRec, setDisposeRec] = useState<InterceptRecord | null>(null)
  const [disposeCmd, setDisposeCmd] = useState('')

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

  // 开处置弹框:初始化可编辑命令为记录原始 command。
  const openDispose = (r: InterceptRecord) => {
    setDisposeRec(r)
    setDisposeCmd(r.command)
  }

  // 加入拦截名单:以处置弹框内编辑后的命令为 match.regex,新建一条 custom 拦截规则。
  // 复用 saveRule('intercept', …)(store Task 14 已建),不新增后端接口。
  // 入参对齐 RuleDTO(saveRule 无 source → POST 创建):match:{regex} 走规则引擎精确匹配。
  const addToBlocklist = async () => {
    if (!disposeRec) return
    const id = `custom.intercept-${Date.now()}`
    await saveRule('intercept', {
      id,
      severity: disposeRec.severity ?? 'high',
      asset_type: '',
      match: { regex: escapeRegex(disposeCmd) },
      enabled: true,
      can_edit: true,
      domain: 'intercept',
    } as Omit<RuleDTO, 'source'> & { source?: 'builtin' | 'custom' })
    message.success(t('intercept.addedToBlocklist', { defaultValue: '已加入拦截名单' }))
    setDisposeRec(null)
  }

  // 加入放行名单:把编辑后的命令追加进 allowlist 全量 PUT。
  // saveAllowlist 全量替换(store SettingsAllowlist 同模式);去重。
  const addToAllowlist = async () => {
    if (!disposeRec) return
    const next = Array.from(new Set([...allowlist, disposeCmd]))
    const ok = await saveAllowlist(next)
    if (ok) message.success(t('intercept.addedToAllowlist', { defaultValue: '已加入放行名单' }))
    setDisposeRec(null)
  }

  const columns: ColumnsType<InterceptRecord> = [
    {
      title: t('intercept.time'),
      dataIndex: 'timestamp',
      width: 170,
      render: (v: string) => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' }}>{formatDateTime(v)}</span>
      ),
    },
    {
      title: t('intercept.outcome'),
      dataIndex: 'outcome',
      width: 90,
      render: (v: string) => <Tag style={{ background: outcomeColor[v] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{v}</Tag>,
    },
    {
      title: t('intercept.command'),
      dataIndex: 'command',
      ellipsis: { showTitle: false },
      render: (v: string) => (
        <Tooltip title={v}><Typography.Text code style={{ fontSize: 'var(--fs-sm)' }}>{v}</Typography.Text></Tooltip>
      ),
    },
    {
      title: t('intercept.rule'),
      dataIndex: 'rule_id',
      width: 160,
      ellipsis: true,
      render: (v?: string) => v ? (
        <Typography.Text code style={{ fontSize: 'var(--fs-sm)' }}>{v}</Typography.Text>
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
        <Tag style={{ background: confidenceColor[v] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{v}</Tag>
      ) : <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: t('intercept.colAction', { defaultValue: '操作' }), width: 180, render: (_: unknown, r: InterceptRecord) => (
        <Space size="small" onClick={(e) => e.stopPropagation()}>
          <Button size="small" onClick={() => openDispose(r)}>{t('intercept.dispose', { defaultValue: '处置' })}</Button>
          <Popconfirm
            title={t('intercept.confirmDelete')}
            okText={t('intercept.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('common.cancel')}
            onConfirm={() => onDelete(r.id)}
          >
            <Button type="text" danger size="small" icon={<DeleteOutlined />} aria-label={t('intercept.delete')} />
          </Popconfirm>
        </Space>
      ),
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
        width={720}
        title={t('intercept.detail')}
        destroyOnClose
      >
        {loadingDetail ? <Spin style={{ display: 'block', margin: '40px auto' }} /> : detail && (
          <Form layout="vertical">
            {/* 时间最上方 */}
            <Form.Item label={t('intercept.time')}>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' }}>{formatDateTime(detail.timestamp)}</span>
            </Form.Item>
            {/* 命令独立行(文字多,独占) */}
            <Form.Item label={t('intercept.command')}>
              <pre style={{ background: 'var(--color-surface)', padding: 8, borderRadius: 4, fontSize: 'var(--fs-sm)', fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0 }}>
                {detail.command}
              </pre>
            </Form.Item>
            {/* 决策 + 严重度 + 置信度(三列同行,重要度高) */}
            <div style={{ display: 'flex', gap: 16 }}>
              <Form.Item label={t('intercept.outcomeLabel', { defaultValue: '决策' })} style={{ flex: 1 }}>
                <Tag style={{ background: outcomeColor[detail.outcome] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{detail.outcome}</Tag>
              </Form.Item>
              <Form.Item label={t('intercept.severityLabel', { defaultValue: '严重度' })} style={{ flex: 1 }}>
                {detail.severity ? <Tag>{detail.severity}</Tag> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
              <Form.Item label={t('intercept.confidenceLabel', { defaultValue: '置信度' })} style={{ flex: 1 }}>
                {detail.confidence ? <Tag style={{ background: confidenceColor[detail.confidence] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{detail.confidence}</Tag> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
            </div>
            {/* 原因(文字可能多,独立行) */}
            {detail.reason ? (
              <Form.Item label={t('intercept.reason')}>
                <Typography.Paragraph style={{ margin: 0 }}>{detail.reason}</Typography.Paragraph>
              </Form.Item>
            ) : null}
            {/* 命中片段(命令级背景,独立行) */}
            {detail.matched_span ? (
              <Form.Item label={t('intercept.matchedSpan')}>
                <pre style={{ background: 'var(--color-surface)', padding: 8, borderRadius: 4, fontSize: 'var(--fs-sm)', fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0 }}>
                  {detail.matched_span}
                </pre>
              </Form.Item>
            ) : null}
            {/* 规则背景信息分组(rule_id/pack_id/tool_name) */}
            <Typography.Title level={5} style={{ marginTop: 8 }}>{t('intercept.ruleBgTitle', { defaultValue: '规则背景' })}</Typography.Title>
            <div style={{ display: 'flex', gap: 16 }}>
              <Form.Item label={t('intercept.rule')} style={{ flex: 1 }}>
                {detail.rule_id ? <Typography.Text code style={{ fontSize: 'var(--fs-sm)' }}>{detail.rule_id}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
              <Form.Item label={t('intercept.packId', { defaultValue: '规则包' })} style={{ flex: 1 }}>
                {detail.pack_id ? <Typography.Text code style={{ fontSize: 'var(--fs-sm)' }}>{detail.pack_id}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
              <Form.Item label={t('intercept.tool')} style={{ flex: 1 }}>
                {detail.tool_name ? <Typography.Text code>{detail.tool_name}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
            </div>
            {/* 执行环境(working_dir/session_id/agent_protocol) */}
            <Typography.Title level={5}>{t('intercept.envTitle', { defaultValue: '执行环境' })}</Typography.Title>
            <Form.Item label={t('intercept.cwd')}>
              {detail.working_dir ? <Typography.Text code style={{ fontSize: 'var(--fs-xs)' }}>{detail.working_dir}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
            </Form.Item>
            <div style={{ display: 'flex', gap: 16 }}>
              <Form.Item label={t('intercept.session')} style={{ flex: 1 }}>
                {detail.session_id ? <Typography.Text code style={{ fontSize: 'var(--fs-xs)' }}>{detail.session_id}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
              <Form.Item label={t('intercept.protocol', { defaultValue: '协议' })} style={{ flex: 1 }}>
                {detail.agent_protocol ? <Typography.Text code style={{ fontSize: 'var(--fs-xs)' }}>{detail.agent_protocol}</Typography.Text> : <Typography.Text type="secondary">—</Typography.Text>}
              </Form.Item>
            </div>
            <Popconfirm
              title={t('intercept.confirmDelete')}
              okText={t('intercept.delete')}
              okButtonProps={{ danger: true }}
              cancelText={t('common.cancel')}
              onConfirm={() => onDelete(detail.id)}
            >
              <Button danger icon={<DeleteOutlined />} style={{ marginTop: 8 }}>
                {t('intercept.delete')}
              </Button>
            </Popconfirm>
          </Form>
        )}
      </Drawer>
      <Modal
        open={!!disposeRec}
        title={t('intercept.disposeTitle', { defaultValue: '处置拦截记录' })}
        onCancel={() => setDisposeRec(null)}
        footer={null}
        destroyOnClose
        width={640}
      >
        {disposeRec ? (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('intercept.command')}</Typography.Text>
              <Input.TextArea
                value={disposeCmd}
                onChange={(e) => setDisposeCmd(e.target.value)}
                rows={3}
                style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)', marginTop: 4 }}
              />
            </div>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('intercept.outcome')}</Typography.Text>
              <div style={{ marginTop: 4 }}><Tag style={{ background: outcomeColor[disposeRec.outcome] ?? 'var(--color-rule-2)', color: 'var(--badge-text)', border: 'none' }}>{disposeRec.outcome}</Tag></div>
            </div>
            <Space>
              <Popconfirm title={t('intercept.confirmAddBlocklist', { defaultValue: '确认以此命令新建拦截规则?' })} okText={t('common.save')} cancelText={t('common.cancel')} onConfirm={addToBlocklist}>
                <Button>{t('intercept.addToBlocklist', { defaultValue: '加入拦截名单' })}</Button>
              </Popconfirm>
              <Popconfirm title={t('intercept.confirmAddAllowlist', { defaultValue: '确认将此命令加入放行名单?' })} okText={t('common.save')} cancelText={t('common.cancel')} onConfirm={addToAllowlist}>
                <Button>{t('intercept.addToAllowlist', { defaultValue: '加入放行名单' })}</Button>
              </Popconfirm>
            </Space>
          </Space>
        ) : null}
      </Modal>
    </Card>
  )
}
