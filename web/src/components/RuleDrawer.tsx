import { Drawer, Descriptions, Typography, Empty, Badge as AntBadge } from 'antd'
import type { DetectorMeta } from '../types'
import { Badge as SevBadge, type BadgeTone } from './Badge'
import { sourceLabel, type FlatRule } from './RulesTable'
import { SEVERITY_LABEL } from '../lib/severity'

interface RuleDrawerProps {
  rule: FlatRule | null
  detectors: DetectorMeta[]
  onClose: () => void
}

// 规则详情抽屉:列表只露规则号/名称/级别/检测器/语法(截断),这里补完整语法 + 所属检测器上下文
// (引擎 / 覆盖范围 / 可用状态)。结构与风险详情抽屉(FindingDrawer)对齐:Descriptions 规则信息 + 检测器区块。
export function RuleDrawer({ rule, detectors, onClose }: RuleDrawerProps) {
  // 所属检测器:按 detector_id 定位,取列表未展示的 engines/covers/available 上下文。
  const detector = rule ? detectors.find((d) => d.id === rule.detector_id) : undefined
  const engines = detector?.engines ?? []
  const covers = detector?.covers ?? []

  return (
    <Drawer
      title="规则详情"
      placement="right"
      width="50%"
      open={rule !== null}
      onClose={onClose}
      maskClosable
      keyboard
      rootClassName="rule-drawer"
      styles={{ body: { padding: 16, overflow: 'auto' } }}
    >
      {rule ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <Descriptions title="规则信息" size="small" column={1} bordered>
            <Descriptions.Item label="规则号">
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 14 }}>{rule.id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="规则名称">{rule.description}</Descriptions.Item>
            <Descriptions.Item label="级别">
              <SevBadge tone={`sev-${rule.severity}` as BadgeTone}>{SEVERITY_LABEL[rule.severity]}</SevBadge>
            </Descriptions.Item>
            <Descriptions.Item label="检测器">{rule.detector}</Descriptions.Item>
            <Descriptions.Item label="来源">{rule.source ? sourceLabel[rule.source] ?? rule.source : '--'}</Descriptions.Item>
            <Descriptions.Item label="校验">
              {rule.valid === false ? (
                <AntBadge status="error" text="无效" />
              ) : (
                <AntBadge status="success" text="有效" />
              )}
            </Descriptions.Item>
            <Descriptions.Item label="规则语法">
              {/* 列表截断展示,详情完整呈现;mono 等宽 + wordBreak 防长正则撑破抽屉,与风险详情抽屉一致。 */}
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 14, wordBreak: 'break-all', color: 'var(--text)' }}>
                {rule.syntax || '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label="资产类型">
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.asset_type || '--'}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="修复建议">
              <span style={{ fontSize: 13 }}>{rule.remediation || '--'}</span>
            </Descriptions.Item>
            <Descriptions.Item label="路径过滤">
              {rule.paths ? (
                <span style={{ fontSize: 12 }}>
                  {rule.paths.include?.length ? `包含: ${rule.paths.include.join(', ')} ` : ''}
                  {rule.paths.exclude?.length ? `排除: ${rule.paths.exclude.join(', ')}` : ''}
                  {!rule.paths.include?.length && !rule.paths.exclude?.length ? '无' : ''}
                </span>
              ) : <Typography.Text type="secondary">无</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label="后置排除">
              {rule.post_exclude?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{rule.post_exclude.join(', ')}</span>
              ) : <Typography.Text type="secondary">无</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label="反混淆">
              {rule.deobfuscation?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.deobfuscation.join(', ')}</span>
              ) : <Typography.Text type="secondary">无</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label="Dotall">{rule.dotall ? '是' : '否'}</Descriptions.Item>
            <Descriptions.Item label="元数据">
              {rule.metadata && Object.keys(rule.metadata).length > 0 ? (
                <pre style={{ margin: 0, fontSize: 11, fontFamily: 'var(--font-mono)' }}>{JSON.stringify(rule.metadata, null, 2)}</pre>
              ) : <Typography.Text type="secondary">无</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label="来源文件">
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11, wordBreak: 'break-all' }}>{rule.source_file || '--'}</Typography.Text>
            </Descriptions.Item>
            {rule.project_path ? (
              <Descriptions.Item label="项目路径">
                <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11, wordBreak: 'break-all' }}>{rule.project_path}</Typography.Text>
              </Descriptions.Item>
            ) : null}
          </Descriptions>

          {/* 所属检测器:列表只露检测器名,这里补引擎 / 覆盖范围 / 可用状态等上下文。 */}
          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>所属检测器</Typography.Title>
            {detector ? (
              <Descriptions size="small" column={1} bordered>
                <Descriptions.Item label="可用状态">
                  <AntBadge status={detector.available ? 'success' : 'error'} text={detector.available ? '可用' : '不可用'} />
                  {!detector.available && detector.reason ? (
                    <Typography.Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>{detector.reason}</Typography.Text>
                  ) : null}
                </Descriptions.Item>
                <Descriptions.Item label="引擎">
                  {engines.length > 0 ? (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      {engines.map((e) => (
                        <div key={e.name} style={{ fontSize: 13 }}>
                          <AntBadge status={e.available ? 'success' : 'error'} />
                          <span style={{ color: 'var(--text)', marginLeft: 4 }}>{e.name}</span>
                          <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)', fontSize: 11, marginLeft: 8 }}>{e.kind}</Typography.Text>
                          {!e.available && e.reason ? (
                            <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>{e.reason}</Typography.Text>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ) : <Typography.Text type="secondary">无</Typography.Text>}
                </Descriptions.Item>
                <Descriptions.Item label="覆盖范围">
                  {covers.length > 0 ? (
                    <span style={{ display: 'inline-flex', flexWrap: 'wrap', gap: 4 }}>
                      {covers.map((c) => <SevBadge key={c} tone="neutral">{c}</SevBadge>)}
                    </span>
                  ) : <Typography.Text type="secondary">全部资产</Typography.Text>}
                </Descriptions.Item>
              </Descriptions>
            ) : <Empty description="未找到检测器" />}
          </div>
        </div>
      ) : null}
    </Drawer>
  )
}
