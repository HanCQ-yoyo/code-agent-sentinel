import { useSyncExternalStore } from 'react'

type Theme = 'light' | 'dark'
const KEY = 'sentinel_theme'

function detect(): Theme {
  const saved = localStorage.getItem(KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return 'light'
}

// 单一共享数据源:模块级 store。旧实现每次调用 useTheme() 都拿到独立 useState,
// TopBar 的 toggle 不会传到 App / AssetDetailPanel → Monaco 主题不跟随。
// 现在所有调用方订阅同一个 store,toggle 即时广播给所有订阅者。
let currentTheme: Theme = detect()
const listeners = new Set<() => void>()

// 模块加载时设置一次 data-theme(等价于旧实现首次 useEffect 跑完后的状态),
// 确保首次渲染前 CSS 变量就已就位。
if (typeof document !== 'undefined') {
  document.documentElement.setAttribute('data-theme', currentTheme)
}

function emitChange() {
  for (const l of listeners) l()
}

function setTheme(next: Theme) {
  if (next === currentTheme) return
  currentTheme = next
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', currentTheme)
  }
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(KEY, currentTheme)
  }
  emitChange()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function getSnapshot(): Theme {
  return currentTheme
}

export function useTheme() {
  // 第三个参数 = server snapshot(SSR 安全;本项目 SPA 不走 SSR,但类型要求且无副作用)。
  // getSnapshot 返回字符串原语,useSyncExternalStore 用 Object.is 比较,字符串按值比较,
  // 同值返回稳定引用 → 不会触发多余重渲染。
  const theme = useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  const toggle = () => setTheme(currentTheme === 'dark' ? 'light' : 'dark')
  return { theme, toggle }
}
