import { Routes, Route, useLocation } from 'react-router-dom'
import { useEffect } from 'react'
import { ConfigProvider, Layout } from 'antd'
import Dashboard from './pages/Dashboard'
import Assets from './pages/Assets'
import Findings from './pages/Findings'
import History from './pages/History'
import Settings from './pages/Settings'
import AssetDetail from './components/AssetDetail'
import { AuthGate } from './components/AuthGate'
import { Sidebar } from './components/Sidebar'
import { TopBar } from './components/TopBar'
import { ThemeShell } from './components/ThemeShell'
import { useTheme } from './theme'
import { antdTheme } from './theme/antdTheme'
import { useStore } from './store'

const { Content } = Layout

const titles: Record<string, string> = {
  '/': '态势看板',
  '/dashboard': '态势看板',
  '/assets': '资产浏览',
  '/findings': '安全发现',
  '/history': '历史扫描',
  '/settings': '设置',
}

export default function App() {
  const { theme } = useTheme()
  const { runScan, loading, detectors, fetchLatestScan } = useStore()
  const loc = useLocation()
  const title = titles[loc.pathname] ?? 'Sentinel'
  useEffect(() => { fetchLatestScan() }, [fetchLatestScan])

  return (
    <ConfigProvider theme={antdTheme(theme)}>
      <AuthGate>
        <Layout style={{ minHeight: '100vh' }}>
          <TopBar title={title} onScan={() => runScan()} loading={loading} detectors={detectors} />
          <Layout>
            <Sidebar />
            <Content style={{ overflow: 'auto', padding: 24 }}>
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/assets" element={<Assets />} />
                <Route path="/assets/:id" element={<AssetDetail />} />
                <Route path="/findings" element={<Findings />} />
                <Route path="/history" element={<History />} />
                <Route path="/history/:id" element={<History />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/__theme" element={<ThemeShell />} />
                <Route path="*" element={<div className="text-text-muted">页面不存在</div>} />
              </Routes>
            </Content>
          </Layout>
        </Layout>
      </AuthGate>
    </ConfigProvider>
  )
}
