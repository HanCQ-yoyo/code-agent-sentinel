import { Routes, Route, useLocation } from 'react-router-dom'
import { useEffect } from 'react'
import { ConfigProvider, Layout } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import { useTranslation } from 'react-i18next'
import Dashboard from './pages/Dashboard'
import Assets from './pages/Assets'
import Findings from './pages/Findings'
import History from './pages/History'
import Settings from './pages/Settings'
import AssetDetail from './components/AssetDetail'
import { AuthGate } from './components/AuthGate'
import { Sidebar } from './components/Sidebar'
import { TopBar } from './components/TopBar'
import { RescanModal } from './components/RescanModal'
import { useTheme } from './theme'
import { antdTheme } from './theme/antdTheme'
import { useStore } from './store'

const { Content } = Layout

export default function App() {
  const { theme } = useTheme()
  const { loading, fetchLatestScan, fetchSettings, fetchPinnedProjects, rescanOpen, rescanInitial, openRescan, closeRescan, selectedAgent } = useStore()
  const { i18n } = useTranslation()
  useEffect(() => { fetchLatestScan() }, [fetchLatestScan, selectedAgent])
  useEffect(() => { fetchSettings() }, [fetchSettings])
  useEffect(() => { fetchPinnedProjects() }, [fetchPinnedProjects])
  const locale = i18n.language === 'en' ? enUS : zhCN

  // 布局:Sider 直接挂根 Layout → 全高;品牌落最左上角。
  // 内层 Layout 顶 TopBar(面包屑 + 操作)+ Content。
  return (
    <ConfigProvider theme={antdTheme(theme)} locale={locale}>
      <AuthGate>
        <Layout style={{ minHeight: '100vh' }}>
          <Sidebar />
          <Layout>
            <TopBar onOpenRescan={() => openRescan()} loading={loading} />
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
                <Route path="*" element={<div className="text-text-muted">页面不存在</div>} />
              </Routes>
            </Content>
          </Layout>
        </Layout>
      </AuthGate>
      <RescanModal open={rescanOpen} onClose={closeRescan} initialScope={rescanInitial} />
    </ConfigProvider>
  )
}
