// 规则结构化表单 + match 树编辑器 e2e —— create/view/builtin 只读核心路径(无外网依赖)。
//
// 覆盖:
//   1. create 新建单叶子规则:填 id/severity/asset_type/field/op/value → 实时校验 success → 保存 → 列表出现。
//   2. view 内置规则(含 and/or/not)树渲染正确(非 JSON 文本)。
//   3. builtin 规则 view 态只读:点行开 view 抽屉 → 「内置规则只读」徽标 + Fork 按钮 + 无 Save 按钮。
//      (注:UI 不为 builtin 规则提供「编辑」入口——Settings.handleEdit 仅绑 custom 行的 Edit 按钮,
//       builtin 行只有 Fork 按钮。故改测 view 态只读标记,等价验证 builtin 防御。)
//
// 离线边界:不验证保存规则在扫描里命中(需扫描子流程,脆弱)。

import { test, expect } from '@playwright/test'

const TOKEN = 'e2e-test-token-123'

test.beforeEach(async ({ page }) => {
  await page.route('**/*.woff2', (r) => r.abort())
  await page.route('**/fonts.googleapis.com/**', (r) => r.abort())
  await page.route('**/fonts.gstatic.com/**', (r) => r.abort())
  await page.addInitScript(() => {
    window.localStorage.setItem('sentinel.lang', 'zh')
  })
})

async function gotoRulesConfig(page: import('@playwright/test').Page) {
  await page.goto(`/#token=${TOKEN}`, { waitUntil: 'domcontentloaded' })
  await page.getByRole('menuitem', { name: /设置/i }).click()
  await page.getByRole('tab', { name: /规则配置|Rules config/ }).click()
}

test('create 新建单叶子规则保存后列表出现', async ({ page }) => {
  await gotoRulesConfig(page)
  await expect(page.locator('.ant-table-row').filter({ visible: true }).first()).toBeVisible({ timeout: 10000 })

  // 点「新建规则」打开 create 抽屉。
  await page.getByRole('button', { name: /新建规则/ }).click()
  // 抽屉内 match 树标题可见(证明结构化表单渲染,非 Monaco)。
  await expect(page.getByText('匹配条件', { exact: true })).toBeVisible({ timeout: 5000 })

  // 填 id(基础区第一个 Input,create 模式可编辑)。
  const idInput = page.locator('.rule-drawer input').first()
  await idInput.fill('custom.e2e-single-leaf')

  // 选 asset_type=settings(让 field 建议表生效)。
  // asset_type Select 在 create 模式空值,placeholder「任意类型」可见。点该 Select 的 selector 开下拉。
  // (不能点 placeholder 文本:被 search input 拦截;点 .ant-select-selector 外框可正确触发。)
  const assetTypeSelector = page.locator('.rule-drawer .ant-select').filter({ hasText: '任意类型' }).locator('.ant-select-selector')
  await assetTypeSelector.click()
  await page.locator('.ant-select-item-option-content', { hasText: 'settings' }).click()

  // match 树叶子:field AutoComplete 填 skip_dangerous,op 选 eq,value 填 true。
  // antd AutoComplete 的 placeholder 在 .ant-select-selection-placeholder span 上;点 .ant-select-selector 开输入。
  await page.locator('.rule-drawer .ant-select').filter({ hasText: '字段名' }).locator('.ant-select-selector').click()
  // AutoComplete 搜索 input:点 selector 后 input 变 active,fill 值。
  await page.locator('.rule-drawer input[role="combobox"]:not([readonly])').first().fill('skip_dangerous')
  // op Select:placeholder「操作」。点 selector 开下拉,选 eq。
  await page.locator('.rule-drawer .ant-select').filter({ hasText: '操作' }).locator('.ant-select-selector').click()
  await page.getByText('eq', { exact: true }).click()
  // value Input:op 选了 eq 后渲染 value Input,placeholder「值」。
  await page.getByPlaceholder('值').fill('true')

  // 实时校验:等待「校验通过」Alert 出现(防抖 500ms + POST /validate)。
  const validateResp = page.waitForResponse(
    (r) => r.url().includes('/api/detect-rules/validate') && r.request().method() === 'POST',
    { timeout: 10000 },
  )
  await validateResp
  await expect(page.getByText('校验通过').first()).toBeVisible({ timeout: 5000 })

  // 保存:点抽屉顶部「保存」按钮 → POST /api/detect-rules。
  const saveResp = page.waitForResponse(
    (r) => r.url().includes('/api/detect-rules') && r.request().method() === 'POST' && r.status() === 200,
    { timeout: 10000 },
  )
  await page.getByRole('button', { name: /保\s?存/ }).click()
  await saveResp

  // 保存成功后列表重拉,新规则行出现。
  await expect(page.locator('.ant-table-row').filter({ hasText: 'custom.e2e-single-leaf' })).toBeVisible({ timeout: 10000 })
})

test('view 内置规则 match 树渲染非纯 JSON', async ({ page }) => {
  await gotoRulesConfig(page)
  await expect(page.locator('.ant-table-row').filter({ visible: true }).first()).toBeVisible({ timeout: 10000 })

  // 点首行 builtin 规则(来源「内置」)的行 → 打开 view 抽屉。
  const builtinRow = page.locator('.ant-table-row').filter({ visible: true, hasText: '内置' }).first()
  await builtinRow.click()
  // view 抽屉的 match 区:含 AND/OR/NOT Tag 或叶子字段(非纯 <pre> JSON)。
  // 选含 or 的内置规则更稳:baseline.wildcard-write-edit 有 or。但分页/顺序不定,
  // 改为断言「规则语法」标签后存在树渲染容器(antd Tag 或 Input,非单一 pre)。
  await expect(page.getByText('规则语法').first()).toBeVisible({ timeout: 5000 })
  // match 区不应是裸 JSON pre(树渲染会产 Tag/Input/Select);断言存在 antd 结构元素。
  const syntaxArea = page.locator('.rule-drawer .ant-descriptions-view').filter({ hasText: '规则语法' })
  await expect(syntaxArea.locator('.ant-tag, .ant-select, .ant-input').first()).toBeVisible({ timeout: 5000 })
})

test('builtin 规则 view 态只读标记与 Fork 入口', async ({ page }) => {
  await gotoRulesConfig(page)
  await expect(page.locator('.ant-table-row').filter({ visible: true, hasText: '内置' }).first()).toBeVisible({ timeout: 10000 })

  // builtin 规则行点击 → 打开 view 抽屉(行 onClick 打开 view,非 edit)。
  // UI 不为 builtin 行提供「编辑」按钮(仅 custom 行有);builtin 行只有「复制为自定义」Fork 按钮。
  const builtinRow = page.locator('.ant-table-row').filter({ visible: true, hasText: '内置' }).first()
  await builtinRow.click()
  // view 抽屉底部:builtin 只读警告徽标 + Fork 入口。
  await expect(page.getByText('内置规则只读').first()).toBeVisible({ timeout: 5000 })
  // view 抽屉顶部 extra 区的 Fork 按钮(scope 到 .rule-drawer 避免匹配表格里所有 builtin 行的 Fork 按钮;
  // 抽屉内 extra 区 + 底部各有 1 个 Fork 按钮,取 first 即 extra 区)。
  await expect(page.locator('.rule-drawer').getByRole('button', { name: /复制为自定义/ }).first()).toBeVisible({ timeout: 3000 })
  // view 模式无 Save 按钮(isEditing=false → extra 只渲染 Fork,不渲染 Save/Cancel)。
  await expect(page.getByRole('button', { name: /^保\s?存$/ })).toHaveCount(0)
})
