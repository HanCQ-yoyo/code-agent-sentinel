// Task 17:规则配置页 e2e —— 域切换 + 启停核心路径(无外网依赖)。
//
// 覆盖 Task 14-17 的端到端接线:
//   1. Settings 域切换 Segmented(detect/intercept)渲染 + 切换后表格按域重拉(store fetch + RulesTable domain)。
//   2. 禁用 builtin 规则的 Switch → toggleRule 调用成功,行内 Switch 反映 disabled。
//
// 重型流程(fork→编辑→重扫命中、拦截启停→guard)在离线 e2e 环境脆弱(需扫描/guard 子流程),
// 按 brief「e2e 优先跑无外网依赖的核心路径」暂缓,见 task-17-sqlite-report.md。
//
// beforeEach 独立于 e2e.spec.ts(Playwright 每个文件 beforeEach 各自独立):复制字体拦截 + 中文注入。

import { test, expect } from '@playwright/test'

const TOKEN = 'e2e-test-token-123'

test.beforeEach(async ({ page }) => {
  // 离线环境字体拦截(同 e2e.spec.ts:Google Fonts 无外网挂起 → page.goto load 超时)。
  await page.route('**/*.woff2', (r) => r.abort())
  await page.route('**/fonts.googleapis.com/**', (r) => r.abort())
  await page.route('**/fonts.gstatic.com/**', (r) => r.abort())
  // 中文渲染(本套 e2e 断言用中文定位器)。
  await page.addInitScript(() => {
    window.localStorage.setItem('sentinel.lang', 'zh')
  })
})

// 导航到「规则配置」tab 的公共步骤:带 token 进首页 → 点设置菜单 → 点「规则配置」tab。
async function gotoRulesConfig(page: import('@playwright/test').Page) {
  await page.goto(`/#token=${TOKEN}`, { waitUntil: 'domcontentloaded' })
  await page.getByRole('menuitem', { name: /设置/i }).click()
  await page.getByRole('tab', { name: /规则配置|Rules config/ }).click()
}

// 定位域切换 Segmented:它含「检测规则」/「拦截规则」文案(来源/级别筛选 Segmented 无此文案)。
// antd Segmented 项可点文案(同 e2e.spec.ts:343 tagSeg.getByText 模式),不用 role=radio(Segmented 非标准 radio)。
function domainSegment(page: import('@playwright/test').Page) {
  return page.locator('.ant-segmented').filter({ hasText: /检测规则|拦截规则/ }).first()
}

test('规则配置页域切换 detect↔intercept 刷新表格', async ({ page }) => {
  await gotoRulesConfig(page)
  // 检测器胶囊行 + 规则表行可见(证明 detect 域规则已加载,RulesTable 默认 domain='detect')。
  await expect(page.getByTestId('detector-chips')).toBeVisible({ timeout: 10000 })
  await expect(page.locator('.ant-table-row').filter({ visible: true }).first()).toBeVisible({ timeout: 10000 })

  // 切到拦截规则域:点击前先挂 response 监听,捕获切换触发的 GET /api/intercept-rules。
  // RulesTable 内部 useEffect 在 domain 变化时调 fetchInterceptRules → store GET /api/intercept-rules。
  const seg = domainSegment(page)
  await expect(seg).toBeVisible()
  const interceptResp = page.waitForResponse(
    (r) => r.url().includes('/api/intercept-rules') && r.request().method() === 'GET' && r.status() === 200,
    { timeout: 10000 },
  )
  await seg.getByText('拦截规则', { exact: true }).click()
  await interceptResp

  // 切回检测规则域:断言 detect-rules API 被重拉(验证双向切换都触发表格重拉)。
  const detectResp = page.waitForResponse(
    (r) => r.url().includes('/api/detect-rules') && r.request().method() === 'GET' && r.status() === 200,
    { timeout: 10000 },
  )
  await seg.getByText('检测规则', { exact: true }).click()
  await detectResp
  // detect 域规则行重新可见。
  await expect(page.locator('.ant-table-row').filter({ visible: true }).first()).toBeVisible({ timeout: 10000 })
})

test('禁用 builtin 检测规则后 Switch 反映 disabled', async ({ page }) => {
  await gotoRulesConfig(page)
  await expect(page.locator('.ant-table-row').filter({ visible: true }).first()).toBeVisible({ timeout: 10000 })

  // 找一行 builtin 规则(来源列「内置」灰 Tag)。规则较多有分页,取首个可见 builtin 行。
  const builtinRow = page.locator('.ant-table-row').filter({ visible: true, hasText: '内置' }).first()
  await expect(builtinRow).toBeVisible({ timeout: 10000 })
  // rule_id 在第一列(Typography.Text code)。读首列文本作为 id。
  const ruleId = (await builtinRow.locator('td').first().innerText()).trim()

  // 行内 Switch(启停):操作列第一个 .ant-switch。builtin 默认 enabled=true → aria-checked=true。
  const switchBtn = builtinRow.locator('.ant-switch').first()
  await expect(switchBtn).toBeVisible()
  await expect(switchBtn).toHaveAttribute('aria-checked', 'true')

  // 点击禁用:点击前挂监听,捕获 PUT /api/detect-rules/:id/enabled 返回 200(验证 toggleRule 真正调了 API)。
  const toggleResp = page.waitForResponse(
    (r) => r.url().includes(`/api/detect-rules/${encodeURIComponent(ruleId)}/enabled`) && r.request().method() === 'PUT' && r.status() === 200,
    { timeout: 10000 },
  )
  await switchBtn.click()
  await toggleResp
  // 禁用后:Switch aria-checked=false(store 已重拉 detectRules,行内状态更新)。
  await expect(switchBtn).toHaveAttribute('aria-checked', 'false', { timeout: 10000 })

  // 复原:重新启用该规则(避免污染后续测试 + 验证反向 toggle 也工作)。
  const reEnableResp = page.waitForResponse(
    (r) => r.url().includes(`/api/detect-rules/${encodeURIComponent(ruleId)}/enabled`) && r.request().method() === 'PUT' && r.status() === 200,
    { timeout: 10000 },
  )
  await switchBtn.click()
  await reEnableResp
  await expect(switchBtn).toHaveAttribute('aria-checked', 'true', { timeout: 10000 })
})
