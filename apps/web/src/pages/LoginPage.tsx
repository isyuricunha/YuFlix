import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { APIError } from '../api'
import { useAuth } from '../auth/AuthContext'
import Brand from '../components/Brand'

export default function LoginPage() {
  const { login, system, systemUnavailable } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [visible, setVisible] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const ready = !!system?.authenticationReady
  const http = window.location.protocol === 'http:'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending || !ready) return
    setPending(true)
    setError('')
    try {
      await login(username.trim(), password)
      setPassword('')
      navigate('/', { replace: true })
    } catch (cause) {
      if (cause instanceof APIError) {
        setError(cause.status === 401 ? 'Usuário ou senha inválidos.' :
          cause.status === 429 ? 'Muitas tentativas. Tente novamente em alguns minutos.' :
          cause.status === 403 ? 'Este endereço não está autorizado para login.' :
          cause.status === 503 ? 'O serviço de autenticação está temporariamente indisponível.' :
          'Não foi possível entrar. Tente novamente.')
      } else {
        setError('Não foi possível conectar ao YuFlix. Verifique a rede.')
      }
    } finally {
      setPending(false)
    }
  }

  return <div className="yf-login">
    <header className="yf-login__header">
      <Brand />
      <span className="yf-login__header-caption">I Watch What I Want.</span>
    </header>

    <main className="yf-login__main">
      <section className="yf-login__intro" aria-label="YuFlix">
        <span className="yf-login__eyebrow">SEU ESPAÇO DE STREAMING</span>
        <h1>Sua tela.<br />Suas escolhas.</h1>
        <p>Filmes, séries, animes e tudo o que você gosta. Um lugar só seu para assistir do seu jeito.</p>
        <div className="yf-login__genres" aria-label="Conteúdos planejados">
          <span>Filmes</span><span>Séries</span><span>Animes</span><span>TV ao vivo</span>
        </div>
      </section>

      <section className="yf-login__access" aria-labelledby="login-title">
        <div className="yf-login__welcome">
          <h2 id="login-title">Bem-vindo de volta.</h2>
          <p>Entre com seu usuário para continuar.</p>
        </div>
        {!ready && <div className="yf-login__availability" role="status">
          <strong>{systemUnavailable ? 'Servidor indisponível' : !system?.databaseReady ? 'Preparando seu acesso' : 'Acesso não configurado'}</strong>
          <p>{systemUnavailable ? 'Não foi possível conectar ao servidor. Atualize a página para tentar novamente.' :
            !system?.databaseReady ? 'O administrador ainda está configurando o banco de dados.' :
            'Este endereço precisa ser autorizado nas configurações do YuFlix.'}</p>
        </div>}

        <form onSubmit={submit} className="yf-login__form" aria-label="Entrar no YuFlix">
          <div className="yf-field">
            <label htmlFor="username">Usuário</label>
            <input id="username" name="username" type="text" autoComplete="username" autoCapitalize="none" spellCheck={false}
              required minLength={3} maxLength={32} placeholder="Nome de usuário"
              value={username} onChange={e => setUsername(e.target.value)} disabled={!ready || pending}/>
          </div>
          <div className="yf-field">
            <label htmlFor="password">Senha</label>
            <div className="yf-field__password">
              <input id="password" name="password" type={visible ? 'text' : 'password'}
                autoComplete="current-password" required placeholder="Sua senha"
                value={password} onChange={e => setPassword(e.target.value)} disabled={!ready || pending}/>
              <button type="button" className="yf-field__reveal" onClick={() => setVisible(v => !v)}
                aria-label={visible ? 'Ocultar senha' : 'Mostrar senha'} aria-pressed={visible} disabled={!ready || pending}>
                {visible ? 'Ocultar' : 'Mostrar'}
              </button>
            </div>
          </div>
          {error && <p className="yf-alert" role="alert">{error}</p>}
          <button type="submit" className="yf-login__submit" disabled={!ready || pending}>
            {pending ? 'Entrando...' : 'Entrar'} <span aria-hidden="true">→</span>
          </button>
        </form>
        {http && <p className="yf-login__network-note">Conexão HTTP sem criptografia. Use apenas em uma rede confiável.</p>}
      </section>
    </main>

    <footer className="yf-login__footer"><span>YuFlix</span><span>Sua experiência. Suas regras.</span></footer>
  </div>
}
