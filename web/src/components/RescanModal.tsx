import { useState, useEffect } from 'react'
import { Modal, Radio, Select, Checkbox, Typography, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { AgentIcon } from './AgentIcon'
import type { Agent } from '../types'

interface Props {
  open: boolean
  onClose: () => void
  initialScope?: { type: string; path?: string } // 页面级入口预填(project 右键菜单)
}

export function RescanModal({ open, onClose, initialScope }: Props) {
  const { t } = useTranslation()
  const { agents, scanEnabledAgents, projects, detectors, runScan, loading } = useStore()
  const [scopeType, setScopeType] = useState('global')
  const [scopePath, setScopePath] = useState<string | undefined>(undefined)
  // 本地多选 agent 状态(非全局 selectedAgents 筛选器)。默认 = scanEnabledAgents。
  // 关闭的 agent(scan_enabled===false)在 Table 行选择中 disabled,不可勾 → 安全检测只扫已开启的。
  const [agentIDs, setAgentIDs] = useState<string[]>([])
  const [detIDs, setDetIDs] = useState<string[]>([])

  // 打开时重置/预填:scope 来自页面入口,agent 默认全选 scanEnabledAgents,检测器默认全选。
  useEffect(() => {
    if (open) {
      setScopeType(initialScope?.type ?? 'global')
      setScopePath(initialScope?.path)
      setAgentIDs(scanEnabledAgents.map(a => a.id))
      setDetIDs((detectors ?? []).map(d => d.id)) // 默认全选
    }
  }, [open, initialScope, scanEnabledAgents, detectors])

  const availDetectors = (detectors ?? []).map(d => ({ label: d.name ?? d.id, value: d.id, disabled: d.available === false }))

  // 目标 Agent 列:名称(AgentIcon + name)、ID、状态(只读 Tag)。
  // 行选择关闭的 agent disabled(只扫已开启的)。状态只读——改状态仍去 Settings。
  const columns: ColumnsType<Agent> = [
    {
      title: t('rescan.colName'), dataIndex: 'name', key: 'name',
      render: (name: string, r: Agent) => (
        <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={r.id} /> {name}</span>
      ),
    },
    { title: t('rescan.colID'), dataIndex: 'id', key: 'id',
      render: (id: string) => <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' }}>{id}</Typography.Text> },
    { title: t('rescan.colStatus'), dataIndex: 'scan_enabled', key: 'status', width: 80,
      render: (on: boolean) => on
        ? <Tag color="green">{t('rescan.statusOn')}</Tag>
        : <Tag>{t('rescan.statusOff')}</Tag> },
  ]

  const start = async () => {
    // 全选检测器=不传(后端全量);否则传逗号分隔 id 列表。
    const det = detIDs.length === (detectors ?? []).length ? undefined : detIDs.join(',')
    // agentIDs 已排除关闭的(Table 行选择 disabled),空 → 后端回退全部 scan_enabled。
    await runScan(agentIDs, det, { type: scopeType, path: scopePath })
    onClose()
  }

  // 三段分区标题:与 AssetDetailPanel 的 .section-label 同款(muted uppercase + hairline),
  // 把原 Space vertical 平铺改成「范围 / 目标 Agent / 检测器」三段分层(design.md #7:消除堆叠感)。
  const SectionLabel = ({ children }: { children: React.ReactNode }) => (
    <div className="section-label" style={{ marginBottom: 8 }}>{children}</div>
  )

  return (
    <Modal open={open} title={t('rescan.title')} onCancel={onClose} onOk={start} okText={t('rescan.start')} confirmLoading={loading} cancelText={t('common.cancel')} width={560}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        {/* 范围 */}
        <section>
          <SectionLabel>{t('rescan.scope')}</SectionLabel>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <Radio.Group value={scopeType} onChange={(e) => setScopeType(e.target.value)}>
              <Radio value="global">{t('rescan.scopeAll')}</Radio>
              <Radio value="user">{t('rescan.scopeUser')}</Radio>
              <Radio value="project">{t('rescan.scopeProject')}</Radio>
            </Radio.Group>
            {scopeType === 'project' ? (
              <Select style={{ flex: 1, minWidth: 180 }} placeholder={t('rescan.selectProject')} value={scopePath}
                options={(projects ?? []).map(p => ({ value: p.path, label: p.name }))} onChange={setScopePath} />
            ) : null}
          </div>
        </section>

        {/* 目标 Agent:行选中背景走 Table.rowSelectedBg(accent-soft,antdTheme 已设),不再泄漏暗色黑底。 */}
        <section>
          <SectionLabel>{t('rescan.agent')}</SectionLabel>
          <Table<Agent>
            size="small" rowKey="id" pagination={false} scroll={{ y: 200 }}
            dataSource={agents?.agents ?? []} columns={columns}
            rowSelection={{
              selectedRowKeys: agentIDs,
              onChange: (keys) => setAgentIDs(keys as string[]),
              // 关闭的 agent 不可勾(只扫已开启的);状态列只读展示其开关。
              getCheckboxProps: (r: Agent) => ({ disabled: !r.scan_enabled }),
            }}
          />
        </section>

        {/* 检测器 */}
        <section>
          <SectionLabel>{t('rescan.detectors')}</SectionLabel>
          <Checkbox.Group value={detIDs} onChange={(v) => setDetIDs(v as string[])} options={availDetectors} style={{ display: 'block' }} />
        </section>
      </div>
    </Modal>
  )
}
