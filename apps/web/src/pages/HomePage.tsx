import { Link, useNavigate } from 'react-router'
import { useState } from 'react'
import Brand from '../components/Brand'
import CatalogHome from '../components/CatalogHome'
import { useAuth } from '../auth/AuthContext'

export default function HomePage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [hasCatalog, setHasCatalog] = useState(false)

  async function signOut() {
    setPending(true)
    setError('')
    try { await logout(); navigate('/login', { replace: true }) }
    catch { setError('Não foi possível encerrar a sessão. Tente novamente.') }
    finally { setPending(false) }
  }

  return <div className="yf-app">
    <header className="yf-app__header">
      <Brand compact />
      <nav className="yf-app__nav" aria-label="Principal">
        <Link to="/" aria-current="page">Início</Link>
        {user?.role === 'admin' && <Link to="/admin">Ajustes</Link>}
      </nav>
      <div className="yf-app__profile">
        <span className="yf-avatar" aria-hidden="true">{user?.username.slice(0,1).toUpperCase()}</span>
        <span className="yf-app__username">{user?.username}</span>
        <button type="button" onClick={signOut} disabled={pending} className="yf-app__logout">Sair</button>
      </div>
    </header>
    <main className="yf-app__main">
      {!hasCatalog && <section className="yf-feature" aria-labelledby="feature-title">
        <div className="yf-feature__spotlight" aria-hidden="true" />
        <div className="yf-feature__copy">
          <span className="yf-feature__eyebrow">YUFLIX · I WATCH WHAT I WANT.</span>
          <h1 id="feature-title">Seu streaming.<br /><span>Seu jeito.</span></h1>
          <p>Aqui, o que assistir é decisão sua. Estamos preparando a biblioteca e os recursos de reprodução.</p>
          {user?.role === 'admin' ? <Link className="yf-feature__action" to="/admin">Abrir ajustes <span aria-hidden="true">→</span></Link> :
            <span className="yf-feature__status">O catálogo ainda está sendo preparado.</span>}
        </div>
        <span className="yf-feature__signature" aria-hidden="true">YUFLIX</span>
      </section>}
      <CatalogHome isAdmin={user?.role === "admin"} onCatalogPresent={setHasCatalog}/>
    </main>
    {error && <p className="yf-alert yf-app__alert" role="alert">{error}</p>}
    <footer className="yf-app__footer"><span>YuFlix</span><span>I Watch What I Want.</span></footer>
  </div>
}
