import { useLayoutEffect, useRef, useState, type ReactNode, ViewTransition } from 'react'
import { NavLink, useLocation, useNavigate, useNavigationType } from 'react-router'
import { useAuth } from '../auth/AuthContext'
import Brand from './Brand'

export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()
  const { pathname } = location
  const navigationType = useNavigationType()
  const positions = useRef(new Map<string, number>())
  const navigate = useNavigate()

  // New screens start at the top. Browser Back/Forward restores the old shelf position.
  useLayoutEffect(() => {
    const y = navigationType === 'POP' ? positions.current.get(location.key) ?? 0 : 0
    window.scrollTo(0, y)
    return () => { positions.current.set(location.key, window.scrollY) }
  }, [location.key, navigationType])
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const isTitle = pathname.startsWith('/title/')

  async function signOut() {
    if (pending) return
    setPending(true)
    setError('')
    try {
      await logout()
      navigate('/login', { replace: true })
    } catch {
      setError('Não foi possível encerrar a sessão. Tente novamente.')
    } finally {
      setPending(false)
    }
  }

  return <div className="yf-shell">
    <a className="yf-skip-link" href="#main-content">Ir para o conteúdo</a>
    <header className="yf-shell__header">
      <Brand compact />
      <nav className="yf-shell__nav" aria-label="Principal">
        <NavLink to="/" end className={({ isActive }) => isActive || isTitle ? 'is-active' : ''}>Início</NavLink>
        {user?.role === 'admin' && <NavLink to="/admin" className={({ isActive }) => isActive ? 'is-active' : ''}>Ajustes</NavLink>}
      </nav>
      <div className="yf-shell__account">
        <span className="yf-avatar" aria-hidden="true">{user?.username.slice(0, 1).toUpperCase()}</span>
        <span className="yf-shell__username">{user?.username}</span>
        <button type="button" onClick={() => void signOut()} disabled={pending} className="yf-shell__logout">
          {pending ? 'Saindo…' : 'Sair'}
        </button>
      </div>
    </header>
    {error && <p role="alert" className="yf-alert yf-shell__error">{error}</p>}
    <ViewTransition name="yf-route">
      <main className="yf-shell__main" id="main-content">{children}</main>
    </ViewTransition>
    <footer className="yf-shell__footer">
      <span>YuFlix</span><span>I Watch What I Want.</span>
    </footer>
  </div>
}
