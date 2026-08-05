import { Button, Popover, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { useEffect, useState } from 'react'

export function GuardCapsule() {
  const { t } = useTranslation()
  const { guardConfig, guardToggling, fetchGuardConfig, toggleGuard } = useStore()
  const [popoverOpen, setPopoverOpen] = useState(false)

  // 初始加载(如果未加载)
  useEffect(() => {
    if (!guardConfig) fetchGuardConfig()
  }, [guardConfig, fetchGuardConfig])

  const enabled = guardConfig?.enabled ?? false
  const loading = guardToggling

  // 开启态:浅绿色填充 + 绿色文字/边框
  const onStyle: React.CSSProperties = {
    color: 'var(--sev-low-solid)',
    borderColor: 'var(--sev-low-solid)',
    background: 'color-mix(in oklch, var(--sev-low-solid) 12%, transparent)',
    fontWeight: 500,
    fontSize: 'var(--fs-sm)',
    padding: '0 12px',
    height: 32,
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    borderRadius: 'var(--radius-input)',
  }

  // 关闭态:浅橙色填充 + 橙色文字/边框
  const offStyle: React.CSSProperties = {
    color: 'var(--sev-high-solid)',
    borderColor: 'var(--sev-high-solid)',
    background: 'color-mix(in oklch, var(--sev-high-solid) 10%, transparent)',
    fontWeight: 500,
    fontSize: 'var(--fs-sm)',
    padding: '0 12px',
    height: 32,
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    borderRadius: 'var(--radius-input)',
  }

  const style = enabled ? onStyle : offStyle

  const label = loading
    ? t('guardCapsule.capsuleToggling')
    : enabled
      ? t('guardCapsule.capsuleOn')
      : t('guardCapsule.capsuleOff')

  const handleConfirmDisable = () => {
    setPopoverOpen(false)
    toggleGuard()
  }

  // 关闭→开启:直接切换(无确认)
  const handleEnable = () => {
    if (loading) return
    toggleGuard()
  }

  // 开启态用 Popover 包装(需要确认关闭)
  if (enabled && !loading) {
    return (
      <Popover
        open={popoverOpen}
        onOpenChange={setPopoverOpen}
        trigger="click"
        placement="bottomRight"
        content={
          <div style={{ maxWidth: 260 }}>
            <Typography.Text style={{ fontSize: 'var(--fs-sm)' }}>
              {t('guardCapsule.confirmOff')}
            </Typography.Text>
            <div style={{ marginTop: 12, textAlign: 'right' }}>
              <Space>
                <Button size="small" onClick={() => setPopoverOpen(false)}>
                  {t('common.cancel')}
                </Button>
                <Button size="small" danger onClick={handleConfirmDisable}>
                  {t('guardCapsule.confirmOk')}
                </Button>
              </Space>
            </div>
          </div>
        }
      >
        <Button style={style}>
          {label}
        </Button>
      </Popover>
    )
  }

  return (
    <Button style={style} onClick={handleEnable} disabled={loading} loading={loading}>
      {label}
    </Button>
  )
}
