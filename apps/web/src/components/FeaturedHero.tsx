import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { CatalogItem, CatalogSource } from '../api'

export type HeroEntry = { item: CatalogItem; source: CatalogSource }

export default function FeaturedHero({ entries }: { entries: HeroEntry[] }) {
  const [index, setIndex] = useState(0)
  const [paused, setPaused] = useState(false)
  const [focused, setFocused] = useState(false)
  const [hovered, setHovered] = useState(false)
  const [reducedMotion, setReducedMotion] = useState(false)

  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    setReducedMotion(query.matches)
    const listener = (event: MediaQueryListEvent) => setReducedMotion(event.matches)
    query.addEventListener('change', listener)
    return () => query.removeEventListener('change', listener)
  }, [])

  useEffect(() => {
    if (entries.length < 2 || reducedMotion || paused || focused || hovered) return
    const timer = window.setInterval(() => {
      if (!document.hidden) setIndex(current => (current + 1) % entries.length)
    }, 10000)
    return () => window.clearInterval(timer)
  }, [entries.length, reducedMotion, paused, focused, hovered])

  if (!entries.length) return null

  const selected = entries[index % entries.length]
  const { item, source } = selected
  const advance = (direction: -1 | 1) => setIndex(current => (current + entries.length + direction) % entries.length)

  return <section className="yf-catalog-featured yf-hero" aria-labelledby="featured-title"
    onMouseEnter={() => setHovered(true)}
    onMouseLeave={() => setHovered(false)}
    onFocusCapture={() => setFocused(true)}
    onBlurCapture={event => {
      if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false)
    }}>
    <div className="yf-hero__picture" key={source.id + ':' + item.id} aria-hidden="true">
      <img className="yf-catalog-featured__backdrop" src={item.background} alt=""
        referrerPolicy="no-referrer" />
    </div>
    <div className="yf-catalog-featured__content yf-hero__content" key={'copy:' + item.id}>
      <span className="yf-kicker">EM DESTAQUE NO CATÁLOGO</span>
      <h2 id="featured-title">{item.name}</h2>
      {item.releaseInfo && <span className="yf-hero__release">{item.releaseInfo}</span>}
      {item.description && <p>{item.description}</p>}
      <Link to={'/title/' + source.id + '/' + item.type + '/' + encodeURIComponent(item.id)}>
        Ver detalhes <span aria-hidden="true">→</span>
      </Link>
    </div>
    {entries.length > 1 && <div className="yf-hero__controls" aria-label="Controles dos destaques">
      <button type="button" className="yf-hero__arrow"
        onClick={() => advance(-1)} aria-label="Destaque anterior">‹</button>
      <span className="yf-hero__count" aria-live="off">{index % entries.length + 1} / {entries.length}</span>
      <button type="button" className="yf-hero__arrow"
        onClick={() => advance(1)} aria-label="Próximo destaque">›</button>
      <button type="button" className="yf-hero__pause"
        aria-pressed={paused} aria-label={paused ? 'Retomar destaques automáticos' : 'Pausar destaques automáticos'}
        onClick={() => setPaused(value => !value)}>
        {paused ? 'Retomar' : 'Pausar'}
      </button>
    </div>}
  </section>
}
