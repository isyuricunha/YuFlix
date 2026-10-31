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
beforeEach(() => { document.title = 'YuFlix' })

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
})
