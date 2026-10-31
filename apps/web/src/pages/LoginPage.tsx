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
  const canLogin = !!system?.authenticationReady
  const http = window.location.protocol === 'http:'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending || !canLogin) return
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

  return <main className="login-layout">
    <section className="login-story" aria-label="Apresentação do YuFlix">
      <header className="login-story__header">
        <Brand />
        <span className="login-story__edition">O SEU STREAMING</span>
      </header>
      <div className="login-story__illustration" aria-hidden="true">
        <span className="login-story__orbit">Y</span>
        <span className="login-story__frame login-story__frame--one" />
        <span className="login-story__frame login-story__frame--two" />
        <span className="login-story__frame login-story__frame--three" />
      </div>
      <div className="login-story__content">
        <div className="story-overline"><span /> SEM LIMITES PARA ESCOLHER</div>
        <h1>O que assistir<br />é <em>decisão sua.</em></h1>
        <p>Filmes, séries e animes no seu espaço. Sem abrir mão do controle.</p>
        <div className="login-story__separator" />
        <div className="login-story__footer">
          <span>YUFLIX</span>
          <span>I WATCH WHAT I WANT.</span>
        </div>
      </div>
    </section>

    <section className="login-entry" aria-labelledby="login-title">
      <div className="login-entry__inner">
        <div className="login-entry__top">
          <span className="entry-label">ACESSO PESSOAL</span>
          <span className="entry-division">01 / 01</span>
        </div>
        <div className="login-entry__head">
          <p className="login-entry__eyebrow">BEM-VINDO AO SEU ESPAÇO</p>
          <h2 id="login-title" aria-label="Bom ter você de volta">Bom ter você<br />de volta.</h2>
          <p>Entre com o seu usuário para continuar.</p>
        </div>
        {!canLogin && <div className="login-availability" role="status">
          <strong>{systemUnavailable ? 'Servidor indisponível' : !system?.databaseReady ? 'Preparando seu acesso' : 'Acesso não configurado'}</strong>
          <p>{systemUnavailable ? 'Não foi possível conectar ao servidor. Tente atualizar a página.' :
            !system?.databaseReady ? 'O administrador ainda está configurando o banco de dados.' :
            'Este endereço precisa ser autorizado pelo administrador nas configurações do YuFlix.'}</p>
        </div>}
        <form onSubmit={submit} className="login-form" aria-label="Entrar no YuFlix">
          <div className="form-field">
            <label htmlFor="username">Usuário</label>
            <input id="username" name="username" type="text" autoComplete="username" autoCapitalize="none"
              spellCheck={false} required minLength={3} maxLength={32} placeholder="Seu nome de usuário"
              value={username} onChange={e => setUsername(e.target.value)} disabled={!canLogin || pending} />
          </div>
          <div className="form-field">
            <label htmlFor="password">Senha</label>
            <div className="password-input">
              <input id="password" name="password" type={visible ? 'text' : 'password'}
                autoComplete="current-password" required placeholder="Sua senha"
                value={password} onChange={e => setPassword(e.target.value)} disabled={!canLogin || pending} />
              <button type="button" className="password-toggle" onClick={() => setVisible(!visible)}
                aria-label={visible ? 'Ocultar senha' : 'Mostrar senha'} aria-pressed={visible}
                disabled={!canLogin}>{visible ? 'Ocultar' : 'Mostrar'}</button>
            </div>
          </div>
          {error && <p className="form-alert" role="alert">{error}</p>}
          <button type="submit" className="login-submit" disabled={!canLogin || pending}>
            <span>{pending ? 'Verificando acesso...' : 'Entrar'}</span>
            <span className="login-submit__arrow" aria-hidden="true">→</span>
          </button>
        </form>
        {http && <p className="http-notice"><span aria-hidden="true">!</span> Acesso por HTTP. Esta conexão não é criptografada. Use uma rede de confiança.</p>}
        <div className="login-entry__bottom">
          <span>Uma conta para cada pessoa.</span>
          <span>I WATCH WHAT I WANT.</span>
        </div>
      </div>
    </section>
  </main>
}
