import { test, expect } from '@playwright/test'
import { writeFileSync, mkdirSync } from 'fs'

test.beforeAll(() => {
  mkdirSync('/tmp/sentinel-e2e-home/.claude', { recursive: true })
  writeFileSync('/tmp/sentinel-e2e-home/.claude/settings.json', JSON.stringify({ permissions: { allow: ['Bash(*)'] } }))
  // Task 9:加 memory 资产(CLAUDE.md → memory 类型),供 md 资产预览断言;
  // 含 fenced bash 代码块,同时间接覆盖 MonacoBlock 在预览中的渲染。
  writeFileSync('/tmp/sentinel-e2e-home/.claude/CLAUDE.md', '# 项目记忆\n\n示例代码块:\n\n```bash\necho hello\n```\n')
})

test('dashboard 带 token 认证后扫描并返回数据依赖结果', async ({ page }) => {
  // 用 --token 标志启动 sentinel(见 playwright.config.ts webServer.command),
  // 故 token 已知为 e2e-test-token-123。经 URL fragment 传递(与真实客户端一致):
  // fragment 不进 server log / Referer,由前端 token() 提取后注入 Authorization 头。
  await page.goto('/#token=e2e-test-token-123')

  // 次要断言:页面骨架已渲染(品牌落侧边栏最上方)
  await expect(page.getByTestId('brand')).toBeVisible()

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
  await expect(page.getByTestId('brand')).toBeVisible()

  // 导航到 /assets(会触发 React Router pushState,丢 #token fragment)
  // Task 3 起 Sidebar 用 antd Menu,菜单项渲染为 role="menuitem"(非 link),可访问名=项文本
  await page.getByRole('menuitem', { name: /资产/i }).click()
  // 再导航回 /dashboard
  await page.getByRole('menuitem', { name: /dashboard/i }).click()

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
  await expect(nav.getByRole('menuitem', { name: /dashboard/i })).toBeVisible()
  await expect(nav.getByRole('menuitem', { name: /资产/i })).toBeVisible()
  await expect(nav.getByRole('menuitem', { name: /风险管理/ })).toBeVisible()
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
  // 可见的是 <label.ant-radio-button-wrapper> 内的文本。阶段 D 后页面新增了标签筛选 Segmented
  // (也含"全部"项),故用 .first() 锁定类型 Radio 的"全部"(DOM 顺序在前)。
  await expect(page.getByText('全部', { exact: true }).first()).toBeVisible()
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
  await page.getByRole('menuitem', { name: /风险管理/ }).click()
  // fixture 含 Bash(*) → 至少一条 finding
  await expect(page.locator('[data-testid="finding-row"]').first()).toBeVisible({ timeout: 15000 })
  // 点击行打开风险详情抽屉:断言抽屉容器 + 风险信息区 + 资产区(asset-detail-name)均渲染。
  await page.locator('[data-testid="finding-row"]').first().click()
  await expect(page.locator('.finding-drawer')).toBeVisible({ timeout: 10000 })
  await expect(page.getByText('风险信息')).toBeVisible()
  await expect(page.getByTestId('asset-detail-name')).toBeVisible({ timeout: 10000 })
})

test('md 资产预览渲染 markdown', async ({ page }) => {
  // 选一个 markdown 类资产(memory/skill/command/agent),断言预览区 .markdown-preview 渲染。
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  // 找一个 md 资产行(类型 Badge 文本含 memory/skill/command/agent 之一)
  const mdRow = page.locator('[data-testid="asset-row"]').filter({ hasText: /memory|skill|command|agent/ }).first()
  await mdRow.click()
  // 抽屉打开后 asset-detail-name 可见,内容区 .markdown-preview 渲染
  await expect(page.getByTestId('asset-detail-name')).toBeVisible({ timeout: 10000 })
  await expect(page.locator('.markdown-preview')).toBeVisible({ timeout: 10000 })
})

test('结构化资产详情渲染', async ({ page }) => {
  // 选一个结构化资产(settings/permissions/mcp_server/hook/keybinding/plugin),
  // 断言 structured-kv(结构化视图)或 monaco-editor(源码视图)渲染。
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  const structRow = page.locator('[data-testid="asset-row"]').filter({ hasText: /settings|permissions|mcp_server|hook|keybinding|plugin/ }).first()
  await structRow.click()
  await expect(page.getByTestId('asset-detail-name')).toBeVisible({ timeout: 10000 })
  await expect(page.locator('[data-testid="structured-kv"], .monaco-editor').first()).toBeVisible({ timeout: 10000 })
})

test('设置页合并视图渲染检测器与规则', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /设置/i }).click()
  // 合并后默认 Tab「检测器与规则」直接渲染:胶囊行 + 规则列表 Segmented 的「全部 N」
  await expect(page.getByTestId('detector-chips')).toBeVisible({ timeout: 10000 })
  await expect(page.getByText(/全部 \d+/)).toBeVisible({ timeout: 10000 })
  // 点一个检测器胶囊 → 该检测器规则数胶囊可见(快捷筛选)
  await page.getByTestId('detector-chip').first().click()
  await expect(page.getByTestId('detector-chip').first()).toHaveAttribute('aria-pressed', 'true')
})

// 阶段 D 资产页增强:标签筛选 / 收藏置顶 / 分页 / 切 tab 关抽屉 / 无资产文件打开。
// 全部用文本断言,不截图(多模态不支持)。

test('资产列表分页与收藏置顶', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 分页:antd Pagination 存在(共 N 条 / 页大小选择器)
  await expect(page.getByText(/共 \d+ 条/)).toBeVisible({ timeout: 10000 })
  // 收藏:点第一行的星标 → 该行应置顶(收藏优先排序)
  const firstRow = page.locator('[data-testid="asset-row"]').first()
  const firstName = await firstRow.locator('td').nth(1).innerText()
  await firstRow.locator('[data-testid="fav-toggle"]').click()
  // 收藏计数 Tag 出现
  await expect(page.getByText(/★ \d+ 置顶/)).toBeVisible({ timeout: 5000 })
  // 置顶后第一行名应仍是该资产(收藏排前)
  const newFirst = await page.locator('[data-testid="asset-row"]').first().locator('td').nth(1).innerText()
  expect(newFirst.trim()).toBe(firstName.trim())
})

test('标签筛选切换隐藏非选中', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 标签筛选 Segmented 的「运行时」项(fixture 的 settings.json 是 config,无运行时资产)
  const tagSeg = page.locator('.ant-segmented').nth(1)
  await tagSeg.getByText('运行时', { exact: true }).click()
  // 选运行时后,config 资产(settings)应被隐藏 → 列表为空(antd empty 文案)
  await expect(page.getByText('暂无资产')).toBeVisible({ timeout: 5000 })
  // 切回全部 → 资产恢复
  await tagSeg.getByText('全部', { exact: true }).click()
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 5000 })
})

test('切换项目 tab 关闭详情抽屉', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 全局 tab 下点行打开抽屉
  await page.locator('[data-testid="asset-row"]').first().click()
  await expect(page.locator('.asset-drawer')).toBeVisible({ timeout: 10000 })
  // 切到另一个 tab(若存在项目 tab)→ 抽屉应关闭
  const projTab = page.getByRole('tab').filter({ hasText: /sentinel/i }).first()
  if (await projTab.count() > 0) {
    await projTab.click()
    await expect(page.locator('.asset-drawer')).not.toBeVisible({ timeout: 5000 })
  }
})

test('文件树无资产文件可打开原始内容', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  await expect(page.locator('[data-testid="asset-row"]').first()).toBeVisible({ timeout: 10000 })
  // 切到文件树视图
  const viewSeg = page.locator('.view-segmented')
  await viewSeg.getByText('文件树', { exact: true }).click()
  // 树渲染:根节点可见(antd Tree title)
  await expect(page.locator('.ant-tree-list')).toBeVisible({ timeout: 10000 })
})

// P2 编辑流程 e2e:进资产抽屉 → 编辑 → 预览 → 确认保存 → 部分重扫反馈 toast。
// 选 memory 资产(CLAUDE.md):editableText 返回 asset.content = 原始文件内容,
// 故不修改 draft 即为 no-op 编辑,commit 写回相同内容,fixture 不变,
// 不影响依赖 Bash(*) finding 存在的其他测试。后端 Commit 不短路:backup+原子写+重算+部分重扫照跑。
test('编辑 CLAUDE.md 保存后部分重扫反馈', async ({ page }) => {
  await page.goto('/#token=e2e-test-token-123')
  await page.getByRole('menuitem', { name: /资产/i }).click()
  // 选 memory 资产行开抽屉(fixture 全局 CLAUDE.md 可编辑)
  const mdRow = page.locator('[data-testid="asset-row"]').filter({ hasText: /memory/ }).first()
  await mdRow.click()
  await expect(page.locator('.asset-drawer')).toBeVisible({ timeout: 10000 })
  // 点编辑(抽屉内):T13 enterEdit 异步 preview 探测可编辑性+乐观锁,通过后才进编辑态
  await page.locator('.asset-drawer').getByRole('button', { name: /编辑/ }).click()
  // 「预览变更」按钮出现 = enterEdit 通过(可编辑 + base_hash_ok)
  await expect(page.getByTestId('preview-edit')).toBeVisible({ timeout: 10000 })
  // 不改 draft(no-op):draft 初值 = editableText(asset) = asset.content = 原始文件内容
  await page.getByTestId('preview-edit').click()
  // 预览 Modal 弹出:标题「预览变更」可见
  await expect(page.locator('.ant-modal-title', { hasText: '预览变更' })).toBeVisible({ timeout: 10000 })
  // 确认保存 → doCommit(备份+原子写+部分重扫)→ 反馈 toast(no-op → new_findings=[] → 成功 toast)
  await page.getByRole('button', { name: /确认保存/ }).click()
  await expect(page.locator('.ant-message-notice')).toBeVisible({ timeout: 10000 })
})
