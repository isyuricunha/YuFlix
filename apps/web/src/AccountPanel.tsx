import { useEffect, useState, type FormEvent } from 'react'

type User = { id: string; username: string; role: 'admin' | 'member'; enabled: boolean }
type State = { user: User }
type Settings = { 'catalog.default_language'?: string; 'playback.prefer_direct'?: boolean }

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch('/api/v1' + path, {
    ...options, credentials: 'same-origin', cache: 'no-store',
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  if (!res.ok) {
    const body: { error?: string } = await res.json().catch(() => ({}))
    throw new Error(body.error || 'Operação indisponível')
  }
  return res.status === 204 ? undefined as T : await res.json() as T
}

export default function AccountPanel({ enabled, databaseReady }: { enabled: boolean; databaseReady: boolean }) {
  const secure = typeof location !== 'undefined' && location.protocol === 'https:'
  const canLogin = enabled
  const [user, setUser] = useState<User | null>(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'member'>('member')
  const [users, setUsers] = useState<User[]>([])
  const [language, setLanguage] = useState('pt-BR')
  const [direct, setDirect] = useState(true)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  useEffect(() => {
    if (!canLogin) return
    let active = true
    api<State>('/auth/me').then(v => { if (active) setUser(v.user) }).catch(() => {})
    return () => { active = false }
  }, [canLogin])

  useEffect(() => {
    if (!user || user.role !== 'admin') return
    let active = true
    Promise.all([api<{ users: User[] }>('/admin/users'), api<{ settings: Settings }>('/admin/settings')])
      .then(([u, v]) => {
        if (!active) return
        setUsers(u.users)
        setLanguage(v.settings['catalog.default_language'] ?? 'pt-BR')
        setDirect(v.settings['playback.prefer_direct'] ?? true)
      }).catch(e => { if (active) setNotice((e as Error).message) })
    return () => { active = false }
  }, [user])

  async function login(e: FormEvent) {
    e.preventDefault()
    setBusy(true); setNotice('')
    try {
      const v = await api<State>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })
      setUser(v.user); setPassword('')
    } catch (e) { setNotice((e as Error).message) }
    finally { setBusy(false) }
  }

  async function logout() {
    setBusy(true); setNotice('')
    try {
      await api('/auth/logout', { method: 'POST' })
      setUser(null); setUsers([])
    } catch (e) { setNotice((e as Error).message) }
    finally { setBusy(false) }
  }

  async function create(e: FormEvent) {
    e.preventDefault()
    setBusy(true); setNotice('')
    try {
      const v = await api<State>('/admin/users', {
        method: 'POST', body: JSON.stringify({ username: newUsername, password: newPassword, role }),
      })
      setUsers(old => [...old, v.user])
      setNewUsername(''); setNewPassword('')
      setNotice('Usuário cadastrado com credenciais independentes.')
    } catch (e) { setNotice((e as Error).message) }
    finally { setBusy(false) }
  }

  async function save(key: string, value: string | boolean) {
    setBusy(true); setNotice('')
    try {
      await api('/admin/settings/' + key, { method: 'PUT', body: JSON.stringify({ value }) })
      setNotice('Preferência armazenada. Aplicação ao catálogo e player ainda em desenvolvimento.')
    } catch (e) { setNotice((e as Error).message) }
    finally { setBusy(false) }
  }

  return <section className="accounts" aria-labelledby="accounts-title">
    <div className="section-heading">
      <div><span className="eyebrow">02 / IDENTIDADE E ACESSO</span><h2 id="accounts-title">{user ? 'Administração' : 'Um espaço para cada pessoa.'}</h2></div>
      {user && <button className="secondary-button" onClick={logout} disabled={busy}>Sair da conta</button>}
    </div>
    {!canLogin ? <div className="accounts-placeholder">
      <span className="eyebrow">02.1 / CONFIGURAÇÃO</span>
      <div><h3>{!databaseReady ? 'Aguardando PostgreSQL 17' : 'Acesso ainda não configurado'}</h3>
        <p>A página pública está disponível. Para ativar contas, configure o banco e uma origem autorizada. HTTP e HTTPS são suportados.</p>
        <small>Em HTTP, o administrador precisa habilitar explicitamente o login e autorizar o endereço local.</small></div>
    </div> : !user ? <form className="accounts-form" onSubmit={login}>
      {!secure && <p className="transport-warning" role="note">Conexão HTTP sem criptografia. Use somente em uma rede confiável: senhas e sessões podem ser interceptadas. HTTPS é recomendado para acesso remoto.</p>}
      <p>Acesso individual por usuário e senha. Fontes compartilhadas, histórico privado.</p>
      <label>Usuário<input autoComplete="username" required minLength={3} maxLength={32} value={username} onChange={e=>setUsername(e.target.value)} /></label>
      <label>Senha<input type="password" autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)} /></label>
      <button className="gold-button" disabled={busy}>Entrar no YuFlix</button>
    </form> : <div className="accounts-dashboard">
      <p>Conectado como <strong>{user.username}</strong> · {user.role === 'admin' ? 'Administrador' : 'Membro'}</p>
      {user.role === 'admin' ? <div className="accounts-columns">
        <div>
          <h3>Usuários</h3>
          <div className="accounts-users">{users.map(v => <div key={v.id}><strong>{v.username}</strong><span>{v.role === 'admin' ? 'Administrador' : 'Membro'}</span></div>)}</div>
          <h4>Adicionar conta</h4>
          <form className="accounts-form" onSubmit={create}>
            <label>Usuário<input required pattern="[a-z][a-z0-9_]{2,31}" autoComplete="off" value={newUsername} onChange={e=>setNewUsername(e.target.value.toLowerCase())} /></label>
            <label>Senha individual<input type="password" required minLength={12} autoComplete="new-password" value={newPassword} onChange={e=>setNewPassword(e.target.value)} /></label>
            <label>Permissão<select value={role} onChange={e=>setRole(e.target.value as 'admin'|'member')}><option value="member">Membro</option><option value="admin">Administrador</option></select></label>
            <button className="gold-button" disabled={busy}>Criar usuário</button>
          </form>
        </div>
        <div><h3>Preferências</h3><p>Persistidas no banco. Ainda não aplicadas ao player.</p>
          <label>Idioma padrão<select value={language} onChange={e=>setLanguage(e.target.value)}><option value="pt-BR">Português (Brasil)</option><option value="en-US">English (US)</option></select></label>
          <button className="secondary-button" disabled={busy} onClick={()=>save('catalog.default_language',language)}>Salvar idioma</button>
          <label className="inline-check"><input type="checkbox" checked={direct} onChange={e=>setDirect(e.target.checked)} /> Priorizar Direct Play</label>
          <button className="secondary-button" disabled={busy} onClick={()=>save('playback.prefer_direct',direct)}>Salvar preferência</button>
        </div>
      </div> : <p>Seu perfil individual está ativo. O catálogo pessoal chegará nas próximas etapas.</p>}
    </div>}
    {notice && <p className="accounts-notice" role="status">{notice}</p>}
  </section>
}
