import { Drawer } from 'antd'
import { useTranslation } from 'react-i18next'
import type { Asset, Finding, DetectorMeta } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

// 遮罩开启(antd mask 默认 true + maskClosable):点击抽屉外遮罩区即关闭,ESC 亦可。
// 抽屉打开时遮罩覆盖左半表格——切看 B 需先点遮罩关闭再点 B 行(与风险/规则详情抽屉一致)。
// rootClassName="asset-drawer":保留 data-testid 钩子供 e2e 定位(Task 15 重写时用 .asset-drawer 选择器)。
// findings/detectors:透传给 AssetDetailPanel 渲染风险列表(基础信息下方 4 列表)。可选,无则不渲染风险区。
// agentID:透传给 AssetDetailPanel 的"安全检查" Modal,scope 到指定 agent(与树视图右栏一致)。
export function AssetDrawer({ asset, findings, detectors, agentID, onClose }: { asset: Asset | null; findings?: Finding[]; detectors?: DetectorMeta[]; agentID?: string; onClose: () => void }) {
  const { t } = useTranslation()
  return (
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
    >
      {asset ? <AssetDetailPanel asset={asset} findings={findings} detectors={detectors} agentID={agentID} /> : null}
    </Drawer>
  )
}
