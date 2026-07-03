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
