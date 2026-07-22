import { useEffect, useState } from 'react'
import { Table, Button, Switch, Input, Modal, Select, Popconfirm, Empty, Card, Space, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { ScheduleStatus } from '../types'

const { Text } = Typography

// Settings 页「定时扫描」tab:列出 / 新建 / inline 改 / 删除 agent 的定时扫描任务。
// 后端 Task 6 提供 /api/schedules CRUD;store Task 12 提供 schedules/create/update/delete。
// schedules 可能从其它源变化(TopBar 不拉 schedules),挂载时无条件刷新(与 agents 的守卫模式不同)。
// 顶部「扫描总开关」Card(Task 4):写 scan_enabled → ScheduleManager.Paused,关掉后所有定时任务暂停;
// scan_interval 仅作无 schedule 时的回退默认,不覆盖已有任务的 interval(以 /api/schedules 为准)。
export function SettingsSchedules() {
  const { t } = useTranslation()
  const { schedules, agents, fetchSchedules, createSchedule, updateSchedule, deleteSchedule, scanEnabled, scanInterval, saveScanToggle } = useStore()
  // 关键适配:store.agents 是 AgentsResponse | null,不是 Agent[]。
  // 与 Task 14 SettingsAgents 一致:取 agents?.agents ?? [] 作为本地 agentList。
  const agentList = agents?.agents ?? []

  useEffect(() => { fetchSchedules() }, [fetchSchedules])

  const [modalOpen, setModalOpen] = useState(false)
  const [newAgent, setNewAgent] = useState<string>('')
  const [newInterval, setNewInterval] = useState<string>('1h')
  const [creating, setCreating] = useState(false)
  // 总开关默认间隔的本地草稿:用受控 value 但仅在失焦/回车时落盘(与 per-agent interval 列
  // 用 defaultValue 不同——此处 scanInterval 来自 store 顶层状态,外部可能变化,需受控同步)。
  const [intervalDraft, setIntervalDraft] = useState<string>(scanInterval)

  useEffect(() => { setIntervalDraft(scanInterval) }, [scanInterval])

  // 已有 schedule 的 agent 不再列入新建候选(后端:每 agent 至多一条 schedule)
  const availableAgents = agentList.filter(a => !schedules.some(s => s.agent_id === a.id))
  // 正在跑(已启用)的 per-agent 任务数,供总开关 hint 展示。
  const runningCount = (schedules ?? []).filter(s => s.enabled).length

  const openModal = () => {
    setNewAgent(availableAgents[0]?.id ?? '')
    setNewInterval('1h')
    setModalOpen(true)
  }
  const handleCreate = async () => {
    if (!newAgent || !newInterval) return
    setCreating(true)
    const ok = await createSchedule(newAgent, newInterval, true)
    setCreating(false)
    if (ok) setModalOpen(false)
  }

  const columns: ColumnsType<ScheduleStatus> = [
    {
      title: t('settings.agentName'),
      dataIndex: 'agent_id',
      render: (id: string) => agentList.find(a => a.id === id)?.name ?? id,
    },
    {
      title: t('settings.enabled'),
      dataIndex: 'enabled',
      render: (v: boolean, record: ScheduleStatus) => (
        <Switch
          size="small"
          checked={v}
          onChange={(checked) => updateSchedule(record.agent_id, record.interval, checked)}
        />
      ),
    },
    {
      title: t('settings.interval'),
      dataIndex: 'interval',
      render: (v: string, record: ScheduleStatus) => (
        <Input
          size="small"
          style={{ width: 120 }}
          defaultValue={v}
          onPressEnter={(e) => {
            const nv = (e.currentTarget as HTMLInputElement).value
            if (nv && nv !== v) updateSchedule(record.agent_id, nv, record.enabled)
          }}
          onBlur={(e) => {
            if (e.target.value && e.target.value !== v) updateSchedule(record.agent_id, e.target.value, record.enabled)
          }}
        />
      ),
    },
    { title: t('settings.lastRun'), dataIndex: 'last_run', render: (v: string) => v || t('common.none') },
    { title: t('settings.nextRun'), dataIndex: 'next_run', render: (v: string) => v || t('common.none') },
    {
      title: t('history.colAction'),
      key: 'action',
      render: (_: unknown, record: ScheduleStatus) => (
        <Popconfirm
          title={t('settings.confirmDeleteSchedule')}
          okText={t('common.delete')}
          okButtonProps={{ danger: true }}
          cancelText={t('common.cancel')}
          onConfirm={() => deleteSchedule(record.agent_id)}
        >
          <Button danger size="small">{t('common.delete')}</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card size="small" style={{ marginBottom: 4 }}>
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Space>
            <Switch size="small" checked={scanEnabled} onChange={(v) => saveScanToggle(v, scanInterval)} />
            <Text strong>{t('settings.scanMasterToggle')}</Text>
          </Space>
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>{t('settings.scanDefaultInterval')}</Text>
            <Input
              size="small"
              style={{ width: 100 }}
              placeholder="30m"
              value={intervalDraft}
              onChange={(e) => setIntervalDraft(e.target.value)}
              onPressEnter={(e) => {
                const nv = (e.currentTarget as HTMLInputElement).value.trim()
                if (nv && nv !== scanInterval) saveScanToggle(scanEnabled, nv)
              }}
              onBlur={(e) => {
                const nv = e.target.value.trim()
                if (nv !== scanInterval) saveScanToggle(scanEnabled, nv)
              }}
            />
          </Space>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {t('settings.scanToggleHint', { running: runningCount })}
          </Text>
        </Space>
      </Card>
      <div>
        <Button type="primary" size="small" onClick={openModal} disabled={availableAgents.length === 0}>
          {t('settings.addSchedule')}
        </Button>
      </div>
      {schedules.length === 0 ? (
        <Empty description={t('settings.noSchedules')} />
      ) : (
        <Table<ScheduleStatus>
          size="small"
          dataSource={schedules}
          rowKey="agent_id"
          pagination={false}
          columns={columns}
        />
      )}
      <Modal
        title={t('settings.addSchedule')}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleCreate}
        okText={t('settings.addSchedule')}
        cancelText={t('common.cancel')}
        confirmLoading={creating}
        okButtonProps={{ disabled: !newAgent || !newInterval }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 8 }}>
          <div>
            <div style={{ marginBottom: 4 }}>{t('settings.agentName')}</div>
            <Select
              value={newAgent}
              onChange={setNewAgent}
              style={{ width: '100%' }}
              placeholder={t('settings.agentName')}
              options={availableAgents.map(a => ({ value: a.id, label: a.name }))}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>{t('settings.interval')}</div>
            <Input
              value={newInterval}
              onChange={(e) => setNewInterval(e.target.value)}
              placeholder="1h / 30m / 2h"
            />
          </div>
        </div>
      </Modal>
    </div>
  )
}
