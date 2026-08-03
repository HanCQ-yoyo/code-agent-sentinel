import { useEffect, useState } from 'react'
import { Input, Button, List, Space, Popconfirm, Typography, message, Empty } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'

export function SettingsAllowlist() {
  const { t } = useTranslation()
  const { allowlist, fetchAllowlist, saveAllowlist } = useStore()
  const fetchGuardConfig = useStore((s) => s.fetchGuardConfig)
  const [input, setInput] = useState('')
  const [list, setList] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => { fetchAllowlist() }, [fetchAllowlist])
  useEffect(() => { fetchGuardConfig() }, [fetchGuardConfig])
  useEffect(() => { setList(allowlist) }, [allowlist])

  const persist = async (newList: string[]) => {
    setSaving(true)
    await saveAllowlist(newList)
    setSaving(false)
  }

  const add = async () => {
    const cmd = input.trim()
    if (!cmd) return
    if (list.includes(cmd)) {
      message.warning(t('guard.allowlistDup'))
      return
    }
    const newList = [...list, cmd]
    setList(newList)
    setInput('')
    await persist(newList)
  }

  const remove = async (cmd: string) => {
    const newList = list.filter((c) => c !== cmd)
    setList(newList)
    await persist(newList)
  }

  return (
    <div>
      <Typography.Text type="secondary" style={{ fontSize: 'var(--fs-sm)' }}>{t('guard.allowlistHint')}</Typography.Text>
      <Space.Compact style={{ width: '100%', margin: '12px 0' }}>
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onPressEnter={add}
          placeholder={t('guard.allowlistPlaceholder')}
        />
        <Button icon={<PlusOutlined />} onClick={add} loading={saving}>{t('guard.add')}</Button>
      </Space.Compact>
      {list.length === 0 ? (
        <Empty description={t('guard.allowlistEmpty')} />
      ) : (
        <List
          bordered
          dataSource={list}
          renderItem={(cmd) => (
            <List.Item
              actions={[
                <Popconfirm
                  key="del"
                  title={t('common.confirmDelete')}
                  okText={t('common.delete')}
                  okButtonProps={{ danger: true }}
                  cancelText={t('common.cancel')}
                  onConfirm={() => remove(cmd)}
                >
                  <Button type="text" danger size="small" icon={<DeleteOutlined />} aria-label={t('common.delete')} />
                </Popconfirm>,
              ]}
            >
              <Typography.Text code>{cmd}</Typography.Text>
            </List.Item>
          )}
        />
      )}
    </div>
  )
}
