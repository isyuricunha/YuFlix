import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { APIError, api, type SettingValues, type User } from '../api'
import { useAuth } from '../auth/AuthContext'
import Brand from '../components/Brand'

type Section = 'overview' | 'users' | 'preferences'

export default function AdminPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [section, setSection] = useState<Section>('overview')
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'member' | 'admin'>('member')
  const [language, setLanguage] = useState<'pt-BR' | 'en-US'>('pt-BR')
  const [preferDirect, setPreferDirect] = useState(true)

  useEffect(() => {
    let active = true
    Promise.all([
      api<{ users: User[] }>('/admin/users'),
      api<{ settings: SettingValues }>('/admin/settings'),
    ]).then(([people, settings]) => {
      if (!active) return
      setUsers(people.users)
      setLanguage(settings.settings['catalog.default_language'] ?? 'pt-BR')
      setPreferDirect(settings.settings['playback.prefer_direct'] ?? true)
    }).catch(() => {
      if (active) setError('Não foi possível carregar as configurações. Tente atualizar a página.')
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [])

  async function createUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    setBusy(true); setError(''); setNotice('')
    try {
      const result = await api<{ user: User }>('/admin/users', {
        method: 'POST', body: JSON.stringify({ username: username.trim(), password, role }),
      })
      setUsers(previous => [...previous, result.user])
      setUsername(''); setPassword(''); setRole('member')
      setNotice('Conta criada. O novo usuário já pode entrar com a própria senha.')
    } catch (cause) {
      setError(cause instanceof APIError && cause.status === 409
        ? 'Não foi possível criar a conta. Confira se o usuário já existe.'
        : 'Não foi possível criar a conta. Confira os dados e tente novamente.')
    } finally { setBusy(false) }
  }

  async function saveSetting(key: 'catalog.default_language' | 'playback.prefer_direct', value: string | boolean) {
    if (busy) return
    setBusy(true); setError(''); setNotice('')
    try {
      await api('/admin/settings/' + key, { method: 'PUT', body: JSON.stringify({ value }) })
      setNotice('Preferência salva. A aplicação ao catálogo e player será ativada nas próximas etapas.')
    } catch {
      setError('Não foi possível salvar esta preferência.')
    } finally { setBusy(false) }
  }

  async function signOut() {
    if (busy) return
    setBusy(true); setError('')
    try { await logout(); navigate('/login', { replace: true }) }
    catch { setError('Não foi possível encerrar a sessão.') }
    finally { setBusy(false) }
  }

  return <div className="yf-admin">
    <header className="yf-admin__topbar">
      <Brand compact />
      <Link to="/" className="yf-admin__back">Voltar ao início</Link>
      <div className="yf-admin__account"><span className="yf-avatar" aria-hidden="true">{user?.username.slice(0,1).toUpperCase()}</span>
        <span>{user?.username}</span>
        <button className="yf-admin__signout" type="button" disabled={busy} onClick={signOut}>Sair</button>
      </div>
    </header>

    <div className="yf-admin__layout">
      <aside className="yf-admin__sidebar">
        <span className="yf-kicker">CONFIGURAÇÕES</span>
        <nav className="yf-admin__nav" aria-label="Configurações">
          {([
            ['overview', 'Visão geral'],
            ['users', 'Usuários'],
            ['preferences', 'Preferências'],
          ] as const).map(([key, label]) =>
            <button type="button" key={key} onClick={() => { setSection(key); setError(''); setNotice('') }}
              className={section === key ? 'is-active' : ''} aria-pressed={section === key}>
              {label}
            </button>)}
        </nav>
      </aside>

      <main className="yf-admin__content">
        <div className="yf-admin__heading">
          <span className="yf-kicker">YUFLIX / ADMINISTRAÇÃO</span>
          <h1>{section === 'overview' ? 'Visão geral' : section === 'users' ? 'Usuários' : 'Preferências'}</h1>
          <p>{section === 'overview' ? 'Gerencie seu espaço e as pessoas que têm acesso a ele.' :
            section === 'users' ? 'Contas individuais e permissões independentes.' :
            'Escolha como seu conteúdo será apresentado e reproduzido.'}</p>
        </div>
        {loading && <p className="yf-admin__loading" role="status">Carregando suas configurações...</p>}

        {!loading && section === 'overview' && <section className="yf-admin__overview" aria-label="Resumo da instalação">
          <div className="yf-admin__summary">
            <span className="yf-kicker">CONTAS ATIVAS</span>
            <div><strong>{users.filter(person => person.enabled).length}</strong><span>pessoas com acesso ao YuFlix</span></div>
          </div>
          <div className="yf-admin__summary-list">
            <div><div><h2>Usuários</h2><p>Gerencie contas e permissões.</p></div>
              <button type="button" onClick={() => setSection('users')}>Gerenciar <span aria-hidden="true">→</span></button></div>
            <div><div><h2>Preferências</h2><p>Idioma e comportamento de reprodução.</p></div>
              <button type="button" onClick={() => setSection('preferences')}>Configurar <span aria-hidden="true">→</span></button></div>
            <div><div><h2>Catálogos e reprodução</h2><p>Integração e player ainda em desenvolvimento.</p></div>
              <span className="yf-admin__unavailable">Em breve</span></div>
          </div>
        </section>}

        {!loading && section === 'users' && <div className="yf-admin__users">
          <section aria-labelledby="users-heading" className="yf-admin__user-list">
            <div className="yf-admin__section-title"><h2 id="users-heading">Pessoas com acesso</h2><span>{users.length}</span></div>
            {users.length ? <div className="yf-admin__rows">
              {users.map(person => <div className="yf-admin__user" key={person.id}>
                <span className="yf-avatar" aria-hidden="true">{person.username.slice(0,1).toUpperCase()}</span>
                <div><strong>{person.username}</strong><small>{person.enabled ? 'Conta ativa' : 'Desativada'}</small></div>
                <span>{person.role === 'admin' ? 'Administrador' : 'Membro'}</span>
              </div>)}
            </div> : <p className="yf-admin__muted">Nenhum usuário cadastrado.</p>}
          </section>
          <section className="yf-admin__new-user" aria-labelledby="create-title">
            <h2 id="create-title">Adicionar pessoa</h2>
            <p>Cada pessoa usa seu próprio usuário e senha.</p>
            <form className="yf-admin__form" onSubmit={createUser}>
              <label htmlFor="create-username">Nome de usuário</label>
              <input id="create-username" autoComplete="off" pattern="[a-z][a-z0-9_]{2,31}" maxLength={32}
                required value={username} onChange={event => setUsername(event.target.value.toLowerCase())}
                placeholder="Nome de usuário" />
              <label htmlFor="create-password">Senha inicial</label>
              <input id="create-password" type="password" autoComplete="new-password" required minLength={12}
                value={password} onChange={event => setPassword(event.target.value)}
                placeholder="Mínimo de 12 caracteres" />
              <label htmlFor="create-role">Permissão</label>
              <select id="create-role" value={role} onChange={event => setRole(event.target.value as 'member' | 'admin')}>
                <option value="member">Membro</option><option value="admin">Administrador</option>
              </select>
              <button className="yf-admin__primary" type="submit" disabled={busy}>{busy ? 'Criando...' : 'Criar usuário'}</button>
            </form>
          </section>
        </div>}

        {!loading && section === 'preferences' && <section className="yf-admin__preferences" aria-labelledby="settings-heading">
          <h2 id="settings-heading">Preferências gerais</h2>
          <p className="yf-admin__muted">Os valores são salvos no banco. A aplicação ao catálogo e ao player depende da conclusão desses recursos.</p>
          <div className="yf-admin__setting">
            <div><h3>Idioma do catálogo</h3><p>Idioma preferido para títulos e descrições.</p></div>
            <div className="yf-admin__setting-action">
              <label htmlFor="catalog-lang" className="yf-sr-only">Idioma</label>
              <select id="catalog-lang" value={language} onChange={event => setLanguage(event.target.value as 'pt-BR' | 'en-US')}>
                <option value="pt-BR">Português (Brasil)</option><option value="en-US">English (US)</option>
              </select>
              <button type="button" disabled={busy} onClick={() => saveSetting('catalog.default_language',language)}>Salvar</button>
            </div>
          </div>
          <div className="yf-admin__setting">
            <div><h3>Priorizar Direct Play</h3><p>Evitar transcode quando o dispositivo for compatível.</p></div>
            <div className="yf-admin__setting-action">
              <label className="yf-admin__toggle"><input type="checkbox" checked={preferDirect} onChange={event => setPreferDirect(event.target.checked)}/>
                <span>{preferDirect ? 'Ativado' : 'Desativado'}</span></label>
              <button type="button" disabled={busy} onClick={() => saveSetting('playback.prefer_direct',preferDirect)}>Salvar</button>
            </div>
          </div>
        </section>}

        {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
        {error && <p role="alert" className="yf-alert">{error}</p>}
      </main>
    </div>
  </div>
}
