import { Routes, Route, NavLink } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Assets from './pages/Assets'
import Findings from './pages/Findings'
import Settings from './pages/Settings'

export default function App() {
  return (
    <div className="min-h-screen flex">
      <nav className="w-48 bg-bg-card border-r border-bg-border p-4 space-y-2">
        {['dashboard','assets','findings','settings'].map(p => (
          <NavLink key={p} to={`/${p}`} className={({isActive}) => `block px-3 py-2 rounded ${isActive ? 'bg-bg-border' : ''}`}>{p}</NavLink>
        ))}
      </nav>
      <main className="flex-1 p-6"><Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/assets" element={<Assets />} />
        <Route path="/findings" element={<Findings />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<div className="text-slate-400">P1 骨架就绪</div>} />
      </Routes></main>
    </div>
  )
}
