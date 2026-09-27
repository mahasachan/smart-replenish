import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { request, seedUser, shortId } from './api'
import Lab from './Lab'
import Shop from './Shop'

type Tab = 'shop' | 'lab'

const userStoreKey = 'smartreplenish.simulator.users'
const readIds = (fallback: string[]) => {
  try { return [...new Set([...fallback, ...JSON.parse(localStorage.getItem(userStoreKey) ?? '[]') as string[]])] }
  catch { return fallback }
}
const saveId = (id: string) => {
  try { localStorage.setItem(userStoreKey, JSON.stringify([id, ...readIds([]).filter((value) => value !== id)])) }
  catch { /* storage is a convenience only */ }
}
const tabFromHash = (): Tab => window.location.hash === '#lab' ? 'lab' : 'shop'

const tabs: { id: Tab; icon: string; label: string; caption: string }[] = [
  { id: 'shop', icon: '🛒', label: 'Shop', caption: 'Customer app' },
  { id: 'lab', icon: '◈', label: 'Decision lab', caption: 'Rules · Jev scores' },
]

function App() {
  const [tab, setTab] = useState<Tab>(tabFromHash)
  const [userIds, setUserIds] = useState(() => readIds([seedUser]))
  const [userId, setUserId] = useState(seedUser)
  const health = useQuery({ queryKey: ['health'], queryFn: () => request<{ status: string }>('/healthz'), refetchInterval: 10_000 })

  useEffect(() => {
    const sync = () => setTab(tabFromHash())
    window.addEventListener('hashchange', sync)
    return () => window.removeEventListener('hashchange', sync)
  }, [])
  useEffect(() => { document.title = tab === 'shop' ? 'SmartReplenish · Shop' : 'SmartReplenish · Decision lab' }, [tab])

  const addUser = (id: string) => {
    saveId(id)
    setUserIds(readIds([seedUser]))
    setUserId(id)
  }

  return (
    <main className="app-shell">
      <nav className="rail" aria-label="Mode">
        <div className="brand-mark">S<span>R</span></div>
        <div className="rail-divider" />
        {tabs.map((item) => (
          <a key={item.id} href={`#${item.id}`} className={`rail-tab ${tab === item.id ? 'active' : ''}`} aria-current={tab === item.id ? 'page' : undefined}>
            <span className="rail-tab-icon">{item.icon}</span>
            <span className="rail-tab-label">{item.label}</span>
            <small>{item.caption}</small>
          </a>
        ))}
        <div className="rail-bottom"><span className="rail-dot" /></div>
      </nav>

      <section className="workspace">
        <header className="topbar">
          <div className="crumb"><span>SMART REPLENISH</span><b>/</b><strong>{tab === 'shop' ? 'Shop' : 'Decision lab'}</strong></div>
          <div className="topbar-right">
            <label className="customer-select"><span>CUSTOMER</span>
              <select value={userId} onChange={(event) => setUserId(event.target.value)}>
                {userIds.map((id) => <option key={id} value={id}>{id === seedUser ? 'Seed customer · Bangkok' : `Customer · ${shortId(id)}`}</option>)}
              </select>
            </label>
            <span className={`connection ${health.isError ? 'offline' : ''}`}><i />{health.isLoading ? 'Connecting' : health.isError ? 'API offline' : 'API connected'}</span>
            <span className="env-tag">LOCAL ENVIRONMENT</span>
          </div>
        </header>
        {tab === 'shop'
          ? <Shop userId={userId} offline={health.isError} />
          : <Lab userId={userId} offline={health.isError} onUserCreated={addUser} />}
      </section>
    </main>
  )
}

export default App
