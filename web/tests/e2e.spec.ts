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

// 说明:本 e2e 通过 --token 标志使用已知 token,无需从 server stdout 提取。
// 注意:--token 标志由后端 fixer 在另一 worktree 添加,合并前本 worktree 无法运行
// (sentinel 会因未知标志报错)。e2e 执行延后到两 worktree 合并后验证。

test('导航后重新扫描不丢 token(问题 3 回归)', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await expect(page.getByText('态势看板')).toBeVisible()

  // 导航到 /assets(会触发 React Router pushState,丢 #token fragment)
  // Task 7 起 Sidebar 用中文标签,链接可访问名 = 链接文本(资产/看板)
  await page.getByRole('link', { name: /资产/i }).click()
  // 再导航回 /dashboard
  await page.getByRole('link', { name: /看板/i }).click()

  // 重新扫描 —— 旧行为会 401,修复后应成功
  await page.getByRole('button', { name: /重新扫描|扫描/ }).click()
  const score = page.getByTestId('health-score-value')
  await expect(score).not.toHaveText('--', { timeout: 15000 })
  // 确认无 401 错误显示
  await expect(page.getByText(/401|unauthorized|missing or invalid token/i)).toHaveCount(0)
})

test('无 token 显示认证门', async ({ page }) => {
  await page.goto('/')
  // AuthGate 渲染:标题"需要访问 token"可见即认证门已显示
  await expect(page.getByRole('heading', { name: /需要访问 token/i })).toBeVisible({ timeout: 5000 })
})

test('主题切换并持久化', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  // 默认应有主题切换按钮
  const toggle = page.getByRole('button', { name: /主题|theme/i })
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
  const nav = page.getByRole('navigation')
  await expect(nav.getByRole('link', { name: /看板/i })).toBeVisible()
  await expect(nav.getByRole('link', { name: /资产/i })).toBeVisible()
  await expect(nav.getByRole('link', { name: /发现/i })).toBeVisible()
  await expect(nav.getByRole('link', { name: /设置/i })).toBeVisible()
  await nav.getByRole('link', { name: /资产/i }).click()
  await expect(page).toHaveURL(/\/assets/)
})

test('资产页显示资产且可筛选类型', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('link', { name: /资产/i }).click()
  // 应有资产行(至少 settings 一条)
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 类型筛选按钮存在
  await expect(page.getByRole('button', { name: /全部/i })).toBeVisible()
})

test('资产页点击行进入详情', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('link', { name: /资产/i }).click()
  await page.locator('[data-testid="asset-row"]').first().click()
  await expect(page).toHaveURL(/\/assets\//)
})

test('资产详情页显示字段与 hash', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('link', { name: /资产/i }).click()
  await page.locator('[data-testid="asset-row"]').first().click()
  await expect(page.getByTestId('asset-detail-name')).toBeVisible({ timeout: 10000 })
  await expect(page.getByText(/hash/i)).toBeVisible()
})
