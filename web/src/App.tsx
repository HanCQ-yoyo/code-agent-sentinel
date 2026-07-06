import { Routes, Route, useLocation } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Assets from './pages/Assets'
import Findings from './pages/Findings'
import Settings from './pages/Settings'
import AssetDetail from './components/AssetDetail'
import { AuthGate } from './components/AuthGate'
import { Sidebar } from './components/Sidebar'
import { TopBar } from './components/TopBar'
import { useStore } from './store'

const titles: Record<string, string> = {
  '/': '态势看板',
  '/dashboard': '态势看板',
  '/assets': '资产浏览',
  '/findings': '安全发现',
  '/settings': '设置',
}

export default function App() {
  const { runScan, loading, detectors } = useStore()
  const loc = useLocation()
  const title = titles[loc.pathname] ?? 'Sentinel'
  return (
    <AuthGate>
      <div className="min-h-screen flex flex-col">
        <TopBar title={title} onScan={() => runScan()} loading={loading} detectors={detectors} />
        <div className="flex flex-1 min-h-0">
          <Sidebar />
          <main className="flex-1 overflow-auto p-6">
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/assets" element={<Assets />} />
              <Route path="/assets/:id" element={<AssetDetail />} />
              <Route path="/findings" element={<Findings />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<div className="text-text-muted">页面不存在</div>} />
            </Routes>
          </main>
        </div>
      </div>
    </AuthGate>
  )
}
