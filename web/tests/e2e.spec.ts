import { test, expect } from '@playwright/test'
import { writeFileSync, mkdirSync } from 'fs'

test.beforeAll(() => {
  mkdirSync('/tmp/sentinel-e2e-home/.claude', { recursive: true })
  writeFileSync('/tmp/sentinel-e2e-home/.claude/settings.json', JSON.stringify({ permissions: { allow: ['Bash(*)'] } }))
})

test('dashboard 带 token 认证后扫描并返回数据依赖结果', async ({ page }) => {
  // 用 --token 标志启动 sentinel(见 playwright.config.ts webServer.command),
  // 故 token 已知为 e2e-test-token-123。经 URL fragment 传递(与真实客户端一致):
  // fragment 不进 server log / Referer,由前端 token() 提取后注入 Authorization 头。
  await page.goto('/#token=e2e-test-token-123')

  // 次要断言:页面骨架已渲染
  await expect(page.getByText('态势看板')).toBeVisible()

  // 触发扫描
  await page.getByRole('button', { name: /重新扫描|扫描/ }).click()

  // 主要断言(数据依赖):健康分值在扫描前为 "--"(未扫描态),
  // 扫描成功后变为具体数值。fixture 含 Bash(*) 基线 finding → 分数 < 100,
  // 故断言分数可见、非 "--"、且非 100,以此证明后端扫描确实执行并返回了真实数据。
  const score = page.getByTestId('health-score-value')
  await expect(score).not.toHaveText('--', { timeout: 15000 })
  await expect(score).not.toHaveText('100')
})

// 说明:本 e2e 通过 --token 标志使用已知 token(见 playwright.config.ts webServer.command),
// 无需从 server stdout 提取。sentinel 由 Playwright webServer 自动启动(reuseExistingServer=true)。

test('导航后重新扫描不丢 token(问题 3 回归)', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await expect(page.getByText('态势看板')).toBeVisible()

  // 导航到 /assets(会触发 React Router pushState,丢 #token fragment)
  // Task 3 起 Sidebar 用 antd Menu,菜单项渲染为 role="menuitem"(非 link),可访问名=项文本
  await page.getByRole('menuitem', { name: /资产/i }).click()
  // 再导航回 /dashboard
  await page.getByRole('menuitem', { name: /看板/i }).click()

  // 重新扫描 —— 旧行为会 401,修复后应成功
  await page.getByRole('button', { name: /重新扫描|扫描/ }).click()
  const score = page.getByTestId('health-score-value')
  await expect(score).not.toHaveText('--', { timeout: 15000 })
  // 确认无 401 错误显示
  await expect(page.getByText(/401|unauthorized|missing or invalid token/i)).toHaveCount(0)
})

test('无 token 显示认证门', async ({ page }) => {
  await page.goto('/')
  // AuthGate 渲染:antd Result 的 title 在 div.ant-result-title 中(非 heading),
  // 故用 getByText 定位"需要访问 token"标题,可见即认证门已显示
  await expect(page.getByText('需要访问 token')).toBeVisible({ timeout: 5000 })
})

test('主题切换并持久化', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  // TopBar 的主题切换是 antd Switch(无 aria-label,checkedChildren=深/unCheckedChildren=浅)。
  // 页面内仅此一个 Switch,故按 role=switch 定位(antd Switch 渲染 role="switch")
  const toggle = page.getByRole('switch')
  await expect(toggle).toBeVisible()
  // 切换后 data-theme 应变化
  const before = await page.locator('html').getAttribute('data-theme')
  await toggle.click()
  const after = await page.locator('html').getAttribute('data-theme')
  expect(before).not.toBeNull()
  expect(after).not.toBeNull()
  expect(before).not.toBe(after)
})

test('看板扫描后显示健康分与严重度分布', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('button', { name: /重新扫描|扫描/ }).click()
  await expect(page.getByTestId('health-score-value')).not.toHaveText('--', { timeout: 15000 })
  // 4 个严重度行(critical/high/medium/low)均渲染出 severity-{s} testid
  await expect(
    page
      .getByTestId('severity-critical')
      .or(page.getByTestId('severity-high'))
      .or(page.getByTestId('severity-medium'))
      .or(page.getByTestId('severity-low'))
  ).toHaveCount(4)
})

test('侧栏导航含 4 项且 active 高亮', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  // Sidebar 用 antd Menu,容器 role="menu",菜单项 role="menuitem"(可访问名=项文本)
  const nav = page.getByRole('menu')
  await expect(nav.getByRole('menuitem', { name: /看板/i })).toBeVisible()
  await expect(nav.getByRole('menuitem', { name: /资产/i })).toBeVisible()
  await expect(nav.getByRole('menuitem', { name: /发现/i })).toBeVisible()
  await expect(nav.getByRole('menuitem', { name: /设置/i })).toBeVisible()
  await nav.getByRole('menuitem', { name: /资产/i }).click()
  await expect(page).toHaveURL(/\/assets/)
})

test('资产页显示资产且可筛选类型', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  // 应有资产行(至少 settings 一条)
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 类型筛选:Radio.Group 的 "全部" 项是 antd Radio.Button。其实际 <input role=radio> 被 antd 视觉隐藏,
  // 可见的是 <label.ant-radio-button-wrapper> 内的文本,故用 getByText 定位可见文本
  await expect(page.getByText('全部', { exact: true })).toBeVisible()
})

test('资产页点击行进入详情', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await page.locator('[data-testid="asset-row"]').first().click()
  // Task 9+14:点击列表行打开抽屉(mask=false 修竞态),URL 不变。
  // 故断言由 toHaveURL 改为"抽屉出现":asset-drawer 可见 + asset-detail-name 可见
  await expect(page.locator('.asset-drawer')).toBeVisible({ timeout: 10000 })
  await expect(page.getByTestId('asset-detail-name')).toBeVisible()
})

test('资产详情页显示字段与 hash', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await page.locator('[data-testid="asset-row"]').first().click()
  // 抽屉打开后 AssetDetailPanel 渲染:asset-detail-name + hash 标签可见
  await expect(page.getByTestId('asset-detail-name')).toBeVisible({ timeout: 10000 })
  await expect(page.getByText(/hash/i)).toBeVisible()
})

test('发现页扫描后展示 finding 行', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('button', { name: /重新扫描|扫描/ }).click()
  await page.getByRole('menuitem', { name: /发现/i }).click()
  // fixture 含 Bash(*) → 至少一条 finding
  await expect(page.locator('[data-testid="finding-row"]').first()).toBeVisible({ timeout: 15000 })
})
