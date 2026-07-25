import { useEffect } from 'react'
import { Table, Switch, Card, Typography, Tooltip } from 'antd'
import { InfoCircleOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { Agent } from '../types'
import { AgentIcon } from '../components/AgentIcon'

// Settings 页「Code Agents」tab:展示已注册 agent。
// agent 的*配置*(id/name/路径/Enabled 加载标志)只读——修改走 `sentinel setup` + 重启。
// 但 per-agent 的*扫描开关*(ScanEnabled)是运行期覆盖(PUT /api/agents/:id),前端可在此切换:
// 关闭后该 agent 不参与定时扫描(定时任务暂停);手动重扫描仍可强制指定。
//
// 只读提示:原用 antd Alert(大色块 + icon + 标题 + 描述,占空间过大)。改为一行内联小字提示——
// InfoCircleOutlined + muted 文案 + 可复制命令 code 块,作为表格上方的脚注,不再喧宾夺主。
export function SettingsAgents() {
  const { t } = useTranslation()
  const { agents, fetchAgents, saveAgentScanEnabled } = useStore()
  // 复用 TopBar 的守卫模式:agents 已加载(TopBar 早一步拉过)则不重复请求。
  useEffect(() => { if (!agents) fetchAgents() }, [agents, fetchAgents])
  // 命令片段(`sentinel setup`)供复制:从 desc 文案里取反引号包裹的命令,无则回退整句。
  const cmd = (t('settings.agentsReadonlyDesc').match(/`([^`]+)`/)?.[1]) ?? 'sentinel setup'
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Card>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)', color: 'var(--color-muted)', fontSize: 'var(--fs-sm)' }}>
          <InfoCircleOutlined style={{ color: 'var(--color-dim)' }} />
          <span>{t('settings.agentsReadonlyHint')}</span>
          <span style={{ color: 'var(--color-dim)' }}>·</span>
          <span>{t('settings.agentsReadonlyDesc').replace(/`([^`]+)`/g, '')}</span>
          <Tooltip title={t('common.copy')}>
            <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' }}>{cmd}</Typography.Text>
          </Tooltip>
        </div>
        <Table
          size="small"
          dataSource={agents?.agents ?? []}
          rowKey="id"
          pagination={false}
          columns={[
            { title: t('settings.agentName'), dataIndex: 'name', render: (name: string, r: Agent) => (
              // agent 名前加品牌 logo(claude-code 用 Claude 橙 SVG,其他 agent 回退方块)。
              // design.md #4:与 Dashboard/Assets/Findings 的 agent 展示统一。
              <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={r.id} /> {name}</span>
            ) },
            { title: 'ID', dataIndex: 'id' },
            { title: t('settings.rootDir'), dataIndex: 'root_dir' },
            { title: t('settings.claudeJson'), dataIndex: 'claude_json' },
            {
              // 扫描开关放末列:左侧是只读配置信息,操作列靠右符合 antd Table 习惯。
              title: t('settings.scanSwitch'),
              dataIndex: 'scan_enabled',
              width: 100,
              render: (enabled: boolean, record: Agent) => (
                <Switch checked={enabled} onChange={(checked) => saveAgentScanEnabled(record.id, checked)} />
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}
