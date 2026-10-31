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

  return <div className="admin-page">
    <aside className="admin-sidebar">
      <Brand compact />
      <div className="admin-sidebar__section">
        <span className="small-caps">GERENCIAMENTO</span>
        <nav aria-label="Configurações" className="admin-nav">
          {([
            ['overview', 'Visão geral', '01'],
            ['users', 'Pessoas e acessos', '02'],
            ['preferences', 'Preferências', '03'],
          ] as const).map(([key, label, number]) =>
            <button key={key} type="button" onClick={() => { setSection(key); setError(''); setNotice('') }}
              className={section === key ? 'admin-nav__item is-current' : 'admin-nav__item'}
              aria-pressed={section === key}>
              <span>{label}</span><span>{number}</span>
            </button>)}
        </nav>
      </div>
      <div className="admin-sidebar__bottom">
        <Link to="/" className="admin-back"><span aria-hidden="true">←</span> Voltar ao YuFlix</Link>
        <div className="admin-profile"><span className="user-avatar" aria-hidden="true">{user?.username.slice(0,1).toUpperCase()}</span>
          <span><strong>{user?.username}</strong><small>Administrador</small></span></div>
      </div>
    </aside>

    <div className="admin-content">
      <header className="admin-content__top">
        <Link className="admin-mobile-back" to="/">← YuFlix</Link>
        <span className="small-caps">CONFIGURAÇÃO DO YUFLIX</span>
        <button type="button" className="text-button" disabled={busy} onClick={signOut}>Sair da conta</button>
      </header>
      <main className="admin-main">
        <div className="admin-heading">
          <p className="story-overline"><span /> PAINEL ADMINISTRATIVO</p>
          <h1>{section === 'overview' ? 'Tudo no seu lugar.' : section === 'users' ? 'Pessoas e acessos.' : 'Do seu jeito.'}</h1>
          <p>{section === 'overview' ? 'Organize a experiência dos seus usuários sem perder o controle.' :
            section === 'users' ? 'Cada pessoa com seu login, suas permissões e seu histórico.' :
            'Preferências de conteúdo e reprodução disponíveis para o seu servidor.'}</p>
        </div>

        {loading && <p className="admin-loading" role="status">Carregando suas configurações...</p>}
        {!loading && section === 'overview' && <div className="admin-overview">
          <div className="admin-overview__metric"><span>ACESSOS CADASTRADOS</span><strong>{users.length.toString().padStart(2, '0')}</strong><small>Contas com credenciais individuais</small></div>
          <div className="admin-overview__metric"><span>FONTES</span><strong className="admin-overview__text">Em preparação</strong><small>Integrações de catálogos e streaming</small></div>
          <div className="admin-overview__metric"><span>REPRODUÇÃO</span><strong className="admin-overview__text">Em preparação</strong><small>Direct Play, remux e HLS</small></div>
          <div className="admin-overview__next">
            <p className="eyebrow">PRÓXIMOS PASSOS</p><h2>Prepare a experiência de quem vai assistir.</h2>
            <p>Você já pode organizar os usuários e salvar preferências. As fontes e o player entrarão em funcionamento quando a reprodução for validada.</p>
            <button type="button" onClick={() => setSection('users')} className="admin-primary-action">Gerenciar usuários <span aria-hidden="true">→</span></button>
          </div>
        </div>}

        {!loading && section === 'users' && <div className="admin-users-layout">
          <section className="admin-users-list" aria-labelledby="users-heading">
            <div className="admin-subhead"><h2 id="users-heading">Contas cadastradas</h2><span>{users.length} {users.length === 1 ? 'pessoa' : 'pessoas'}</span></div>
            {users.length === 0 ? <p>Nenhum usuário cadastrado.</p> : <div className="admin-users-table">
              {users.map(person => <div className="admin-user-row" key={person.id}>
                <span className="user-avatar" aria-hidden="true">{person.username[0]?.toUpperCase()}</span>
                <div><strong>{person.username}</strong><small>{person.enabled ? 'Conta ativa' : 'Conta desativada'}</small></div>
                <span className="user-role">{person.role === 'admin' ? 'Administrador' : 'Membro'}</span>
              </div>)}
            </div>}
          </section>
          <section className="admin-create" aria-labelledby="create-title">
            <p className="eyebrow">NOVO ACESSO</p><h2 id="create-title">Adicionar pessoa</h2>
            <p>Crie um acesso próprio para cada usuário. Não é necessário compartilhar senhas.</p>
            <form onSubmit={createUser} className="admin-form">
              <label htmlFor="create-username">Nome de usuário</label>
              <input id="create-username" autoComplete="off" required pattern="[a-z][a-z0-9_]{2,31}" maxLength={32} value={username}
                onChange={e => setUsername(e.target.value.toLowerCase())} placeholder="exemplo" />
              <label htmlFor="create-password">Senha inicial</label>
              <input id="create-password" type="password" autoComplete="new-password" minLength={12} required value={password}
                onChange={e => setPassword(e.target.value)} placeholder="Mínimo de 12 caracteres" />
              <label htmlFor="create-role">Permissão</label>
              <select id="create-role" value={role} onChange={e => setRole(e.target.value as 'member' | 'admin')}>
                <option value="member">Membro</option><option value="admin">Administrador</option>
              </select>
              <button className="admin-primary-action" type="submit" disabled={busy}>{busy ? 'Criando acesso...' : 'Criar usuário'} <span aria-hidden="true">→</span></button>
            </form>
          </section>
        </div>}

        {!loading && section === 'preferences' && <section className="admin-preferences" aria-labelledby="settings-heading">
          <div className="admin-subhead"><h2 id="settings-heading">Preferências gerais</h2><span>Valores compartilhados</span></div>
          <p className="admin-preferences__description">As escolhas abaixo já são salvas no banco. Elas serão aplicadas aos conteúdos e ao player quando esses recursos estiverem disponíveis.</p>
          <div className="admin-setting">
            <div><h3>Idioma do catálogo</h3><p>Idioma preferido para títulos e descrições.</p></div>
            <div className="admin-setting__control">
              <label className="sr-only" htmlFor="catalog-lang">Idioma preferido</label>
              <select id="catalog-lang" value={language} onChange={e => setLanguage(e.target.value as 'pt-BR' | 'en-US')}>
                <option value="pt-BR">Português (Brasil)</option><option value="en-US">English (US)</option>
              </select>
              <button type="button" className="admin-outline-action" disabled={busy} onClick={() => saveSetting('catalog.default_language', language)}>Salvar</button>
            </div>
          </div>
          <div className="admin-setting">
            <div><h3>Priorizar reprodução direta</h3><p>Evitar recodificação quando o dispositivo for compatível.</p></div>
            <div className="admin-setting__control">
              <label className="switch-label"><input type="checkbox" checked={preferDirect} onChange={e => setPreferDirect(e.target.checked)}/><span>{preferDirect ? 'Ativado' : 'Desativado'}</span></label>
              <button type="button" className="admin-outline-action" disabled={busy} onClick={() => saveSetting('playback.prefer_direct',preferDirect)}>Salvar</button>
            </div>
          </div>
        </section>}
        {notice && <p className="admin-notice" role="status">{notice}</p>}
        {error && <p className="form-alert" role="alert">{error}</p>}
      </main>
      <footer className="admin-footer">YuFlix · I Watch What I Want.</footer>
    </div>
  </div>
}
