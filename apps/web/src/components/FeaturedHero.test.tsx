import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import FeaturedHero from './FeaturedHero'

const source = {
  id: '123e4567-e89b-12d3-a456-426614174000',
  name: 'Catálogo',
  enabled: true,
  catalogs: [],
}

const entries = [
  { source, item: { id: 'tt001', type: 'movie' as const, name: 'Filme A',
    description: 'Primeira história', background: 'https://example.com/a.jpg' } },
  { source, item: { id: 'tt002', type: 'movie' as const, name: 'Filme B',
    description: 'Segunda história', background: 'https://example.com/b.jpg' } },
]

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('provides discoverable next/previous navigation for real catalog highlights', () => {
  render(<MemoryRouter><FeaturedHero entries={entries} /></MemoryRouter>)
  expect(screen.getByRole('heading', { name: 'Filme A' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Próximo destaque' }))
  expect(screen.getByRole('heading', { name: 'Filme B' })).toBeTruthy()
  expect(screen.getByRole('link', { name: /Ver detalhes/ }).getAttribute('href')).toContain('tt002')
  fireEvent.click(screen.getByRole('button', { name: 'Destaque anterior' }))
  expect(screen.getByRole('heading', { name: 'Filme A' })).toBeTruthy()
})

it('lets viewers pause automatic hero changes and keeps the state accessible', () => {
  render(<MemoryRouter><FeaturedHero entries={entries} /></MemoryRouter>)
  const pause = screen.getByRole('button', { name: 'Pausar destaques automáticos' })
  fireEvent.click(pause)
  const resume = screen.getByRole('button', { name: 'Retomar destaques automáticos' })
  expect(resume.getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(resume)
  expect(screen.getByRole('button', { name: 'Pausar destaques automáticos' }).getAttribute('aria-pressed')).toBe('false')
})

it('does not display carousel controls when only one highlight is available', () => {
  render(<MemoryRouter><FeaturedHero entries={entries.slice(0, 1)} /></MemoryRouter>)
  expect(screen.queryByRole('button', { name: 'Próximo destaque' })).toBeNull()
  expect(screen.getByRole('heading', { name: 'Filme A' })).toBeTruthy()
})
