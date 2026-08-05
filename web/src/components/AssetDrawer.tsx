import { useState } from 'react'
import { Drawer, Button, Modal, Checkbox, Space, Typography } from 'antd'
import { SecurityScanOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { Asset, Finding, DetectorMeta } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

// 遮罩开启(antd mask 默认 true + maskClosable):点击抽屉外遮罩区即关闭,ESC 亦可。
// 抽屉打开时遮罩覆盖左半表格——切看 B 需先点遮罩关闭再点 B 行(与风险/规则详情抽屉一致)。
// rootClassName="asset-drawer":保留 data-testid 钩子供 e2e 定位(Task 15 重写时用 .asset-drawer 选择器)。
// findings/detectors:透传给 AssetDetailPanel 渲染风险列表(基础信息下方 4 列表)。可选,无则不渲染风险区。
// agentID:透传给 AssetDetailPanel 的"安全检查" Modal,scope 到指定 agent(与树视图右栏一致)。
//
// 安全检查按钮移入 Drawer extra header 右对齐;Modal 由 AssetDrawer 拥有,AssetDetailPanel hideCheckButton。
export function AssetDrawer({ asset, findings, detectors, agentID, onClose }: { asset: Asset | null; findings?: Finding[]; detectors?: DetectorMeta[]; agentID?: string; onClose: () => void }) {
  const { t } = useTranslation()
  const { runScan } = useStore()
  const storeDetectors = useStore((s) => s.detectors)
  const [checkOpen, setCheckOpen] = useState(false)
  const [checkDets, setCheckDets] = useState<string[]>([])

  const openCheck = () => {
    setCheckDets((storeDetectors ?? []).map(d => d.id))
    setCheckOpen(true)
  }
  const startCheck = async () => {
    if (!asset) return
    const det = checkDets.length === (storeDetectors ?? []).length ? undefined : checkDets.join(',')
    await runScan(agentID ? [agentID] : [], det, { type: 'asset-id', path: asset.id })
    setCheckOpen(false)
  }

  return (
    <>
      <Drawer
        title={t('assetDrawer.title')}
        placement="right"
        width="50%"
        open={asset !== null}
        onClose={onClose}
        maskClosable
        keyboard
        rootClassName="asset-drawer"
        styles={{ body: { padding: 16, overflow: 'auto' } }}
        extra={
          asset ? (
            <Button size="small" icon={<SecurityScanOutlined />} onClick={openCheck}>
              {t('rescan.check')}
            </Button>
          ) : undefined
        }
      >
        {asset ? <AssetDetailPanel asset={asset} findings={findings} detectors={detectors} agentID={agentID} hideCheckButton /> : null}
      </Drawer>
      <Modal
        open={checkOpen}
        title={t('rescan.checkTitle')}
        onCancel={() => setCheckOpen(false)}
        onOk={startCheck}
        okText={t('rescan.start')}
        cancelText={t('common.cancel')}
        getContainer={false}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Text type="secondary">{t('rescan.checkHint')}</Typography.Text>
          <div>
            <Typography.Text strong>{t('rescan.detectors')}</Typography.Text>
            <Checkbox.Group
              value={checkDets}
              onChange={(v) => setCheckDets(v as string[])}
              options={(storeDetectors ?? []).map(d => ({ label: d.name ?? d.id, value: d.id, disabled: d.available === false }))}
              style={{ display: 'block', marginTop: 4 }}
            />
          </div>
        </Space>
      </Modal>
    </>
  )
}
