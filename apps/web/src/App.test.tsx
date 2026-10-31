import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import App from './App'

const system = {
  name: 'YuFlix',
  slogan: 'I Watch What I Want.',
  apiVersion: 'v1',
  phase: 'foundation',
  playbackReady: false,
  databaseReady: true,
  authenticationReady: true,
}

const admin = {
  id: 'test-admin',
  username: 'owner',
  role: 'admin',
  enabled: true,
  createdAt: '2026-01-01T00:00:00Z',
}

const member = { ...admin, id: 'test-member', username: 'member', role: 'member' }

function fakeResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status, headers: { 'Content-Type': 'application/json' },
  })
}

function serve(options: {
  session?: typeof admin
  success?: boolean
  authenticationReady?: boolean
  catalog?: boolean
  providerDenied?: boolean
  streamSettings?: boolean
  egressAvailable?: boolean
} = {}) {
  const fakeFetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = input.toString()
    if (path.endsWith('/system/info')) return fakeResponse({
      ...system, authenticationReady: options.authenticationReady ?? true,
    })
    if (path.endsWith('/auth/me')) return options.session
      ? fakeResponse({ user: options.session })
      : fakeResponse({ error: 'sessão necessária' }, 401)
    if (path.endsWith('/auth/login')) {
      const payload = JSON.parse(String(init?.body ?? '{}'))
      return options.success && payload.username === 'owner' && payload.password === 'a secure test password'
        ? fakeResponse({ user: admin })
        : fakeResponse({ error: 'usuário ou senha inválidos' }, 401)
    }
    if (path.endsWith('/catalog/sources')) return fakeResponse({ sources: options.catalog ? [{
      id:'123e4567-e89b-12d3-a456-426614174000',name:'Cinemeta',enabled:true,
      catalogs:[{type:'movie',id:'top',name:'Popular'}],
    }] : [] })
    if (path.includes('/catalog/sources/') && path.includes('/items?')) return fakeResponse({
      items:[{id:'tt0133093',type:'movie',name:'The Matrix',
        poster:'https://images.metahub.space/poster/small/tt0133093/img',
        background:'https://images.metahub.space/background/medium/tt0133093/img',
        description:'A science fiction film.'}],
    })
    if (path.includes('/catalog/sources/') && path.includes('/meta?')) return fakeResponse({
      meta: path.includes('type=series')
        ? {id:'tt0903747',type:'series',name:'Breaking Bad',releaseInfo:'2008',
          videos:[{id:'tt0903747:1:1',title:'Piloto',season:1,episode:1},
            {id:'tt0903747:1:2',title:'Segundo episódio',season:1,episode:2}]}
        : {id:'tt0133093',type:'movie',name:'The Matrix',
          description:'A science fiction film.',releaseInfo:'1999',genres:['Sci-Fi']},
    })
    if (path.includes('/media/streams?') && options.providerDenied) return fakeResponse({
      streams: [],providersChecked:1,providersFailed:1,
      providerStatuses:[{name:'BestCine',state:'error',reason:'access_denied',httpStatus:403,received:0,supported:0}],
    })
    if (path.includes('/media/streams?')) return fakeResponse({
      streams:[{sourceId:'123e4567-e89b-12d3-a456-426614174000',
        sourceName:'Test Addon',index:0,key:'opaque-selection',name:'MP4 seguro',mode:'direct_play_candidate'}],
      providersChecked:1,providersFailed:0,
    })
    if (path.endsWith('/media/sessions')) return fakeResponse({
      path:'/api/v1/media/play/opaque-ticket-without-provider-credentials',
      mode:'direct_play',status:'probe_passed_not_browser_verified',
      name:'MP4 seguro',sourceName:'Test Addon',
    },201)
    if (path.endsWith('/media/addons')) return fakeResponse({sources:options.streamSettings ? [{
      id:'123e4567-e89b-12d3-a456-426614174000',name:'Test Provider',
      enabled:true,networkRoute:'direct',
    }] : []})
    if (path.endsWith('/admin/network/egress')) return fakeResponse({
      availableForSources:options.egressAvailable ? {'123e4567-e89b-12d3-a456-426614174000':true} : {},
    })
    if (path.endsWith('/route') && init?.method==='PUT') {
      const selection=JSON.parse(String(init.body??'{}'))
      return fakeResponse({source:{id:'123e4567-e89b-12d3-a456-426614174000',
        name:'Test Provider',enabled:true,networkRoute:selection.route}})
    }

    if (path.endsWith('/admin/users')) return fakeResponse({ users: [admin, member] })
    if (path.endsWith('/admin/settings')) return fakeResponse({ settings: {} })
    return fakeResponse({ error: 'not found' }, 404)
  })
  vi.stubGlobal('fetch', fakeFetch)
  return fakeFetch
}

function route(path: string) {
  return render(<MemoryRouter initialEntries={[path]}><App /></MemoryRouter>)
}

afterEach(() => { cleanup(); vi.unstubAllGlobals() })
beforeEach(() => { document.title = 'YuFlix'; vi.stubGlobal('scrollTo', vi.fn()) })

describe('YuFlix authentication experience', () => {
  it('shows a dedicated sign-in screen for unauthenticated visitors', async () => {
    serve()
    route('/')
    expect(await screen.findByRole('heading', { name: /bem-vindo de volta/i })).toBeTruthy()
    expect(screen.getByRole('textbox', { name: 'Usuário' })).toBeTruthy()
    expect(screen.getByLabelText('Senha')).toBeTruthy()
    expect(screen.queryByText('Contas cadastradas')).toBeNull()
  })

  it('does not leak authentication details when the password is incorrect', async () => {
    serve()
    const actor = userEvent.setup()
    route('/login')
    await screen.findByRole('heading', { name: /bem-vindo de volta/i })
    await actor.type(screen.getByRole('textbox', { name: 'Usuário' }), 'owner')
    await actor.type(screen.getByLabelText('Senha'), 'incorrect passphrase')
    await actor.click(screen.getByRole('button', { name: 'Entrar' }))
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Usuário ou senha inválidos.')
  })

  it('supports showing and hiding the password without changing its value', async () => {
    serve()
    const actor = userEvent.setup()
    route('/login')
    await screen.findByRole('heading', { name: /bem-vindo de volta/i })
    const field = screen.getByLabelText('Senha') as HTMLInputElement
    await actor.type(field, 'some passphrase')
    await actor.click(screen.getByRole('button', { name: 'Mostrar senha' }))
    expect(field.type).toBe('text')
    expect(field.value).toBe('some passphrase')
    await actor.click(screen.getByRole('button', { name: 'Ocultar senha' }))
    expect(field.type).toBe('password')
  })

  it('logs in and navigates to the library, preserving server cookie sessions', async () => {
    const fetcher = serve({ success: true })
    const actor = userEvent.setup()
    route('/login')
    await screen.findByRole('heading', { name: /bem-vindo de volta/i })
    await actor.type(screen.getByRole('textbox', { name: 'Usuário' }), 'owner')
    await actor.type(screen.getByLabelText('Senha'), 'a secure test password')
    await actor.click(screen.getByRole('button', { name: 'Entrar' }))
    expect(await screen.findByRole('heading', { level: 1, name: /seu streaming.*seu jeito/i })).toBeTruthy()
    const calls = fetcher.mock.calls.filter(call => String(call[0]).endsWith('/auth/login'))
    expect(calls).toHaveLength(1)
    expect(calls[0][1]?.credentials).toBe('same-origin')
  })

  it('keeps administration unavailable to regular members', async () => {
    serve({ session: member })
    route('/admin')
    const main = await screen.findByRole('heading', { level: 1, name: /seu streaming.*seu jeito/i })
    expect(main).toBeTruthy()
    expect(screen.queryByText('Pessoas e acessos.')).toBeNull()
  })

  it('restores an existing administrator session on reload', async () => {
    serve({ session: admin })
    route('/admin')
    expect(await screen.findByRole('heading', { name: /visão geral/i })).toBeTruthy()
    await waitFor(() => expect(screen.getByText('CONTAS ATIVAS')).toBeTruthy())
  })

  it('does not show a usable login form when the origin is not authorized', async () => {
    serve({ authenticationReady: false })
    route('/login')
    expect(await screen.findByText('Acesso não configurado')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Entrar' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('displays titles with accurate media availability messaging', async () => {
    serve({session:admin,catalog:true})
    route('/')
    expect(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'})).toBeTruthy()
    expect(screen.getByText(/disponibilidade de reprodução depende das fontes/i)).toBeTruthy()
  })

  it('filters real catalog shelves without inventing series', async () => {
    serve({session:admin,catalog:true})
    const actor=userEvent.setup()
    route('/')
    await screen.findByRole('link',{name:'Ver detalhes de The Matrix'})
    const filter=screen.getByRole('group',{name:'Filtrar títulos'})
    await actor.click(filter.querySelector('[aria-pressed="false"]:last-child') as HTMLButtonElement)
    expect(screen.queryByRole('link',{name:'Ver detalhes de The Matrix'})).toBeNull()
    expect(screen.getByText(/Nenhum título desta categoria/)).toBeTruthy()
    await actor.click(screen.getByRole('button',{name:'Tudo'}))
    expect(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'})).toBeTruthy()
  })

  it('opens catalog metadata details without losing the global navigation', async () => {
    serve({session:admin,catalog:true})
    const actor = userEvent.setup()
    route('/')
    const card = await screen.findByRole('link',{name:'Ver detalhes de The Matrix'})
    await actor.click(card)
    expect(await screen.findByRole('heading',{level:1,name:'The Matrix'})).toBeTruthy()
    expect(screen.getByRole('heading',{name:'Fontes de vídeo'})).toBeTruthy()
    expect(screen.getByRole('navigation',{name:'Principal'})).toBeTruthy()
    await actor.click(screen.getByRole('link',{name:'Início'}))
    expect(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'})).toBeTruthy()
  })

  it('discovers video candidates and uses only same-origin media session URLs', async () => {
    const calls=serve({session:admin,catalog:true})
    const actor=userEvent.setup()
    route('/')
    await actor.click(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'}))
    await screen.findByRole('heading',{name:'The Matrix',level:1})
    await actor.click(screen.getByRole('button',{name:'Buscar fontes'}))
    await actor.click(await screen.findByRole('button',{name:'Testar Direct Play'}))
    const player=await screen.findByLabelText('Reprodutor YuFlix')
    expect(player.getAttribute('src')).toMatch(/^\/api\/v1\/media\/play\//)
    const sessions=calls.mock.calls.filter(call=>String(call[0]).endsWith('/media/sessions'))
    expect(sessions).toHaveLength(1)
    expect(sessions[0][1]?.credentials).toBe('same-origin')
    expect(screen.queryByText(/https:\/\/video-provider/)).toBeNull()
  })

  it('selects an episode before looking up sources and keeps the choice visible', async () => {
    const calls = serve({session:admin})
    const actor = userEvent.setup()
    route('/title/123e4567-e89b-12d3-a456-426614174000/series/tt0903747')
    await screen.findByRole('heading',{name:'Breaking Bad'})
    expect(screen.getByRole('heading',{name:'Episódios'})).toBeTruthy()
    expect(screen.getByText('Escolha um episódio')).toBeTruthy()
    const episode=screen.getByRole('button',{name:/Piloto/})
    await actor.click(episode)
    expect(episode.getAttribute('aria-pressed')).toBe('true')
    await screen.findByRole('heading',{name:'Fontes de vídeo'})
    expect(calls.mock.calls.filter(call => String(call[0]).includes('/media/streams?'))).toHaveLength(0)
    await actor.click(screen.getByRole('button',{name:'Buscar fontes'}))
    await waitFor(()=>expect(calls.mock.calls.some(call=>String(call[0]).includes('tt0903747%3A1%3A1'))).toBe(true))
  })

  it('opens the player as a focused dialog and closes it with Escape', async () => {
    serve({session:admin,catalog:true})
    const actor=userEvent.setup()
    route('/')
    await actor.click(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'}))
    await actor.click(screen.getByRole('button',{name:'Buscar fontes'}))
    const action=await screen.findByRole('button',{name:'Testar Direct Play'})
    await actor.click(action)
    const dialog=await screen.findByRole('dialog',{name:/Reprodução: MP4 seguro/})
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect((document.activeElement as HTMLElement).getAttribute('aria-label')).toBe('Fechar player')
    await actor.keyboard('{Escape}')
    await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button',{name:'Testar Direct Play'})).toBeTruthy()
  })

  it('deep links to a specific administration section', async () => {
    serve({session:admin,streamSettings:true})
    route('/admin?section=streams')
    expect(await screen.findByRole('heading',{name:'Provedores de vídeo'})).toBeTruthy()
    expect(screen.getByRole('button',{name:'Reprodução'}).getAttribute('aria-current')).toBe('page')
  })

  it('explains an upstream 403 without exposing provider URLs', async () => {
    serve({session:admin,catalog:true,providerDenied:true})
    const actor=userEvent.setup()
    route('/')
    await actor.click(await screen.findByRole('link',{name:'Ver detalhes de The Matrix'}))
    await screen.findByRole('heading',{name:'The Matrix',level:1})
    await actor.click(screen.getByRole('button',{name:'Buscar fontes'}))
    expect(await screen.findByText(/Acesso recusado pelo provedor \(HTTP 403\)/)).toBeTruthy()
    expect(screen.queryByText('1 provedor(es) não responderam.')).toBeNull()
    expect(screen.queryByText(/manifest\.json/)).toBeNull()
  })

  it('defaults provider networking to direct without requiring egress', async () => {
    serve({session:admin,streamSettings:true})
    const actor=userEvent.setup()
    route('/admin')
    await actor.click(await screen.findByRole('button',{name:'Reprodução'}))
    const selector=await screen.findByLabelText('Rede') as HTMLSelectElement
    expect(selector.value).toBe('direct')
    expect(selector.querySelectorAll('option')).toHaveLength(1)
    expect(selector.textContent).toContain('Direto')
  })

  it('permits configured egress per addon and persists the mode through admin API', async () => {
    const calls=serve({session:admin,streamSettings:true,egressAvailable:true})
    const actor=userEvent.setup()
    route('/admin')
    await actor.click(await screen.findByRole('button',{name:'Reprodução'}))
    const selector=await screen.findByLabelText('Rede') as HTMLSelectElement
    expect(selector.querySelectorAll('option')).toHaveLength(3)
    await actor.selectOptions(selector,'auto')
    await waitFor(()=>expect(selector.value).toBe('auto'))
    const saves=calls.mock.calls.filter(call=>String(call[0]).endsWith('/route'))
    expect(saves).toHaveLength(1)
    expect(JSON.parse(String(saves[0][1]?.body)).route).toBe('auto')
    expect(saves[0][1]?.credentials).toBe('same-origin')
  })

})
