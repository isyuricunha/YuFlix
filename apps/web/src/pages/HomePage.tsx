import { Link, useNavigate } from 'react-router'
import { useState } from 'react'
import Brand from '../components/Brand'
import { useAuth } from '../auth/AuthContext'

export default function HomePage() {
  const { user, logout, system } = useAuth()
  const navigate = useNavigate()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  async function signOut() {
    setPending(true); setError('')
    try { await logout(); navigate('/login', { replace: true }) }
    catch { setError('Não foi possível encerrar a sessão. Tente novamente.') }
    finally { setPending(false) }
  }

  return <div className="app-page">
    <header className="app-header">
      <div className="app-header__left">
        <Brand compact />
        <nav aria-label="Principal" className="app-nav">
          <Link to="/" aria-current="page">Início</Link>
          {user?.role === 'admin' && <Link to="/admin">Administração</Link>}
        </nav>
      </div>
      <div className="app-header__right">
        <span className="user-avatar" aria-hidden="true">{user?.username.slice(0, 1).toUpperCase()}</span>
        <span className="app-header__username">{user?.username}</span>
        <button className="app-signout" type="button" disabled={pending} onClick={signOut}>Sair</button>
      </div>
    </header>
    <main>
      <section className="home-feature">
        <div className="home-feature__rail"><span className="small-caps">YUFLIX ORIGINAL EXPERIENCE</span><span>001</span></div>
        <div className="home-feature__art" aria-hidden="true"><span>Y</span></div>
        <div className="home-feature__content">
          <p className="story-overline"><span /> SEU ESPAÇO DE ENTRETENIMENTO</p>
          <h1 aria-label="Seu streaming. Seu jeito.">Seu streaming.<br /><em>Seu jeito.</em></h1>
          <p>Uma biblioteca feita para você. Estamos construindo os primeiros recursos para reunir seus filmes, séries e animes.</p>
          {user?.role === 'admin'
            ? <Link className="primary-link" to="/admin">Abrir administração <span aria-hidden="true">→</span></Link>
            : <span className="home-feature__coming">O catálogo estará disponível em breve.</span>}
        </div>
        <span className="home-feature__index" aria-hidden="true">I WATCH WHAT I WANT.</span>
      </section>

      <section className="library-empty" aria-labelledby="empty-title">
        <div className="library-empty__header"><div><p className="eyebrow">SUA BIBLIOTECA</p><h2 id="empty-title">O melhor ainda está por vir.</h2></div>
          <span className="library-empty__state">EM PREPARAÇÃO</span></div>
        <div className="library-empty__body">
          <div className="library-empty__icon" aria-hidden="true"><span className="play-symbol" /></div>
          <div><h3>A biblioteca está sendo preparada.</h3>
            <p>Os catálogos e a reprodução ainda não foram habilitados. Nenhum título ou fonte será exibido como disponível sem passar pelos testes de reprodução.</p>
            {user?.role === 'admin' && <Link to="/admin">Gerenciar contas e preferências <span aria-hidden="true">→</span></Link>}
          </div>
        </div>
      </section>
    </main>
    {error && <p className="form-alert footer-alert" role="alert">{error}</p>}
    <footer className="app-footer"><span>YuFlix · I Watch What I Want.</span><span>{system?.databaseReady ? 'Sua conta está ativa' : 'Conexão em preparação'}</span></footer>
  </div>
}
