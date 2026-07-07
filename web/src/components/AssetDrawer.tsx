import { Drawer } from 'antd'
import type { Asset } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

// mask={false}:抽屉占右侧 50% 无遮罩,左半表格保持可点击——点 B 行直接切内容,修"先关 A 再开 B"竞态。
// rootClassName="asset-drawer":保留 data-testid 钩子供 e2e 定位(Task 15 重写时用 .asset-drawer 选择器)。
export function AssetDrawer({ asset, onClose }: { asset: Asset | null; onClose: () => void }) {
  return (
    <Drawer
      title="资产详情"
      placement="right"
      width="50%"
      open={asset !== null}
      onClose={onClose}
      mask={false}
      keyboard
      rootClassName="asset-drawer"
      styles={{ body: { padding: 16, overflow: 'auto' } }}
    >
      {asset ? <AssetDetailPanel asset={asset} /> : null}
    </Drawer>
  )
}
