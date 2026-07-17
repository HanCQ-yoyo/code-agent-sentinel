import { Drawer, Descriptions, Typography, Empty, Badge as AntBadge } from 'antd'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  // 所属检测器:按 detector_id 定位,取列表未展示的 engines/covers/available 上下文。
  const detector = rule ? detectors.find((d) => d.id === rule.detector_id) : undefined
  const engines = detector?.engines ?? []
  const covers = detector?.covers ?? []

  return (
    <Drawer
      title={t('ruleDrawer.title')}
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
          <Descriptions title={t('ruleDrawer.infoTitle')} size="small" column={1} bordered>
            <Descriptions.Item label={t('ruleDrawer.ruleId')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 14 }}>{rule.id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.ruleName')}>{rule.description}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.severity')}>
              <SevBadge tone={`sev-${rule.severity}` as BadgeTone}>{SEVERITY_LABEL[rule.severity]}</SevBadge>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.detector')}>{rule.detector}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.source')}>{rule.source ? t(sourceLabel[rule.source] ?? '') || rule.source : '--'}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.validity')}>
              {rule.valid === false ? (
                <AntBadge status="error" text={t('ruleDrawer.invalid')} />
              ) : (
                <AntBadge status="success" text={t('ruleDrawer.valid')} />
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.ruleSyntax')}>
              {/* 列表截断展示,详情完整呈现;mono 等宽 + wordBreak 防长正则撑破抽屉,与风险详情抽屉一致。 */}
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 14, wordBreak: 'break-all', color: 'var(--text)' }}>
                {rule.syntax || '--'}
              </span>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.assetType')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.asset_type || '--'}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.remediation')}>
              <span style={{ fontSize: 13 }}>{rule.remediation || '--'}</span>
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.pathFilter')}>
              {rule.paths ? (
                <span style={{ fontSize: 12 }}>
                  {rule.paths.include?.length ? `${t('ruleDrawer.pathInclude', { items: rule.paths.include.join(', ') })} ` : ''}
                  {rule.paths.exclude?.length ? `${t('ruleDrawer.pathExclude', { items: rule.paths.exclude.join(', ') })}` : ''}
                  {!rule.paths.include?.length && !rule.paths.exclude?.length ? t('ruleDrawer.none') : ''}
                </span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.postExclude')}>
              {rule.post_exclude?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{rule.post_exclude.join(', ')}</span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.deobfuscation')}>
              {rule.deobfuscation?.length ? (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{rule.deobfuscation.join(', ')}</span>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.dotall')}>{rule.dotall ? t('common.yes') : t('common.no')}</Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.metadata')}>
              {rule.metadata && Object.keys(rule.metadata).length > 0 ? (
                <pre style={{ margin: 0, fontSize: 11, fontFamily: 'var(--font-mono)' }}>{JSON.stringify(rule.metadata, null, 2)}</pre>
              ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t('ruleDrawer.sourceFile')}>
              <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11, wordBreak: 'break-all' }}>{rule.source_file || '--'}</Typography.Text>
            </Descriptions.Item>
            {rule.project_path ? (
              <Descriptions.Item label={t('ruleDrawer.projectPath')}>
                <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 11, wordBreak: 'break-all' }}>{rule.project_path}</Typography.Text>
              </Descriptions.Item>
            ) : null}
          </Descriptions>

          {/* 所属检测器:列表只露检测器名,这里补引擎 / 覆盖范围 / 可用状态等上下文。 */}
          <div>
            <Typography.Title level={5} style={{ marginTop: 8 }}>{t('ruleDrawer.detectorTitle')}</Typography.Title>
            {detector ? (
              <Descriptions size="small" column={1} bordered>
                <Descriptions.Item label={t('ruleDrawer.available')}>
                  <AntBadge status={detector.available ? 'success' : 'error'} text={detector.available ? t('ruleDrawer.availableYes') : t('ruleDrawer.availableNo')} />
                  {!detector.available && detector.reason ? (
                    <Typography.Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>{detector.reason}</Typography.Text>
                  ) : null}
                </Descriptions.Item>
                <Descriptions.Item label={t('ruleDrawer.engines')}>
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
                  ) : <Typography.Text type="secondary">{t('ruleDrawer.none')}</Typography.Text>}
                </Descriptions.Item>
                <Descriptions.Item label={t('ruleDrawer.covers')}>
                  {covers.length > 0 ? (
                    <span style={{ display: 'inline-flex', flexWrap: 'wrap', gap: 4 }}>
                      {covers.map((c) => <SevBadge key={c} tone="neutral">{c}</SevBadge>)}
                    </span>
                  ) : <Typography.Text type="secondary">{t('ruleDrawer.coversAll')}</Typography.Text>}
                </Descriptions.Item>
              </Descriptions>
            ) : <Empty description={t('ruleDrawer.notFound')} />}
          </div>
        </div>
      ) : null}
    </Drawer>
  )
}
