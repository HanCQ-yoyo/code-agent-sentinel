import { Drawer } from 'antd'
import type { Asset } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

// 遮罩开启(antd mask 默认 true + maskClosable):点击抽屉外遮罩区即关闭,ESC 亦可。
// 抽屉打开时遮罩覆盖左半表格——切看 B 需先点遮罩关闭再点 B 行(与风险/规则详情抽屉一致)。
// rootClassName="asset-drawer":保留 data-testid 钩子供 e2e 定位(Task 15 重写时用 .asset-drawer 选择器)。
export function AssetDrawer({ asset, onClose }: { asset: Asset | null; onClose: () => void }) {
  return (
    <Drawer
      title="资产详情"
      placement="right"
      width="50%"
      open={asset !== null}
      onClose={onClose}
      maskClosable
      keyboard
      rootClassName="asset-drawer"
      styles={{ body: { padding: 16, overflow: 'auto' } }}
    >
      {asset ? <AssetDetailPanel asset={asset} /> : null}
    </Drawer>
  )
}
