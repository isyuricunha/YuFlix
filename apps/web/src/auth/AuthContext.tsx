import {
  createContext, useContext, useEffect, useMemo, useState,
  type ReactNode,
} from 'react'
import { api, type SystemInfo, type User } from '../api'

type AuthState = 'checking' | 'guest' | 'authenticated'
type Session = {
  state: AuthState
  user: User | null
  system: SystemInfo | null
  systemUnavailable: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}
const AuthContext = createContext<Session | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>('checking')
  const [user, setUser] = useState<User | null>(null)
  const [system, setSystem] = useState<SystemInfo | null>(null)
  const [systemUnavailable, setSystemUnavailable] = useState(false)

  useEffect(() => {
    let active = true
    async function restoreSession() {
      let info: SystemInfo
      try {
        info = await api<SystemInfo>('/system/info')
      } catch {
        if (active) { setState('guest'); setSystemUnavailable(true) }
        return
      }
      if (!active) return
      setSystem(info)
      if (!info.authenticationReady) { setState('guest'); return }

      try {
        const session = await api<{ user: User }>('/auth/me')
        if (active) { setUser(session.user); setState('authenticated') }
      } catch {
        if (active) { setUser(null); setState('guest') }
      }
    }
    void restoreSession()
    return () => { active = false }
  }, [])

  async function login(username: string, password: string) {
    const data = await api<{ user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    setUser(data.user)
    setState('authenticated')
  }

  async function logout() {
    await api<void>('/auth/logout', { method: 'POST' })
    setUser(null)
    setState('guest')
  }

  const value = useMemo(() => ({
    state, user, system, systemUnavailable, login, logout,
  }), [state, user, system, systemUnavailable])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): Session {
  const value = useContext(AuthContext)
  if (!value) throw new Error('AuthProvider is required')
  return value
}
