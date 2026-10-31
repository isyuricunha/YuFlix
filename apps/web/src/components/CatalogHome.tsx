import { startTransition, useEffect, useRef, useState, ViewTransition, type Dispatch, type SetStateAction } from 'react'
import { Link } from 'react-router'
import { api, type CatalogSource, type CatalogDefinition, type CatalogItem } from '../api'

type Shelf = { source: CatalogSource; catalog: CatalogDefinition; items: CatalogItem[]; failed?: boolean }
type Kind = 'all' | 'movie' | 'series'

function MediaShelf({ shelf }: { shelf: Shelf }) {
  const rail = useRef<HTMLDivElement>(null)
  if (!shelf.items.length) return null
  const name = shelf.catalog.name + ' de ' + shelf.source.name
  function scroll(direction: -1 | 1) {
    const element = rail.current
    if (!element) return
    element.scrollBy({ left: direction * Math.max(300, element.clientWidth * .75), behavior: 'smooth' })
  }
  return <section className="yf-media-shelf" aria-label={name}>
    <div className="yf-media-shelf__heading">
      <div className="yf-media-shelf__label">
        <span className="yf-kicker">{shelf.source.name}</span>
        <h2>{shelf.catalog.name}</h2>
      </div>
      <div className="yf-media-shelf__controls">
        <span>{shelf.catalog.type === 'movie' ? 'Filmes' : 'Séries'}</span>
        <button type="button" onClick={() => scroll(-1)} aria-label={'Voltar capas: ' + name}>‹</button>
        <button type="button" onClick={() => scroll(1)} aria-label={'Avançar capas: ' + name}>›</button>
      </div>
    </div>
    <div className="yf-media-shelf__rail" ref={rail} aria-label={'Capas: ' + name}>
      {shelf.items.map(item =>
        <Link className="yf-media-card" key={item.id} to={'/title/' + shelf.source.id + '/' + item.type + '/' + encodeURIComponent(item.id)}
          aria-label={'Ver detalhes de ' + item.name}>
          <div className="yf-media-card__image">
            {item.poster && <img src={item.poster} alt="" loading="lazy" referrerPolicy="no-referrer" />}
            {!item.poster && <span aria-hidden="true">{item.name.slice(0, 1)}</span>}
          </div>
          <strong>{item.name}</strong>
          {item.releaseInfo && <small>{item.releaseInfo}</small>}
        </Link>)}
    </div>
  </section>
}

export default function CatalogHome({ isAdmin, onCatalogPresent }: { isAdmin: boolean; onCatalogPresent: Dispatch<SetStateAction<boolean>> }) {
  const [sources, setSources] = useState<CatalogSource[]>([])
  const [shelves, setShelves] = useState<Shelf[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [kind, setKind] = useState<Kind>('all')

  useEffect(() => {
    let active = true
    async function load() {
      try {
        const response = await api<{ sources: CatalogSource[] }>('/catalog/sources')
        if (!active) return
        setSources(response.sources)
        onCatalogPresent(response.sources.length > 0)
        const chosen = response.sources.filter(source => source.enabled)
          .flatMap(source => source.catalogs.filter(c => c.type === 'movie' || c.type === 'series')
            .slice(0, 2).map(catalog => ({ source, catalog }))).slice(0, 4)
        if (!chosen.length) { setLoading(false); return }
        const all = await Promise.all(chosen.map(async item => {
          try {
            const params = '?type=' + encodeURIComponent(item.catalog.type) + '&catalog=' + encodeURIComponent(item.catalog.id)
            const data = await api<{ items: CatalogItem[] }>('/catalog/sources/' + item.source.id + '/items' + params)
            return { ...item, items: data.items }
          } catch { return { ...item, items: [], failed: true } }
        }))
        if (active) { setShelves(all); setLoading(false) }
      } catch {
        if (active) { setError('Não foi possível carregar o catálogo.'); setLoading(false) }
      }
    }
    void load()
    return () => { active = false }
  }, [onCatalogPresent])

  const first = shelves.find(s => s.items.some(i => i.background))
  const featured = first?.items.find(i => i.background)
  const visible = shelves.filter(s => kind === 'all' || s.catalog.type === kind)

  if (loading) return <section className="yf-library yf-library--loading" role="status">Carregando sua biblioteca...</section>
  if (error) return <section className="yf-library" role="alert">{error}</section>
  if (!sources.length) return <section className="yf-library" aria-labelledby="library-title">
    <div className="yf-library__heading"><div><span className="yf-kicker">SUA BIBLIOTECA</span>
      <h2 id="library-title">Tudo em um lugar.</h2></div><span className="yf-library__phase">Em preparação</span></div>
    <div className="yf-library__empty"><div className="yf-library__glyph" aria-hidden="true"><span>▶</span></div>
      <div className="yf-library__message"><h3>Conecte seu primeiro catálogo.</h3>
        <p>Os títulos serão exibidos aqui quando um administrador cadastrar uma fonte. Catálogo não significa reprodução disponível.</p></div>
      {isAdmin && <Link to="/admin?section=catalogs">Configurar fontes <span aria-hidden="true">→</span></Link>}
    </div>
  </section>

  return <div className="yf-catalog-browse">
    {featured && first && <section className="yf-catalog-featured" aria-labelledby="featured-title">
      {featured.background && <img className="yf-catalog-featured__backdrop" src={featured.background} alt="" referrerPolicy="no-referrer" />}
      <div className="yf-catalog-featured__content">
        <span className="yf-kicker">EM DESTAQUE NO CATÁLOGO</span>
        <h2 id="featured-title">{featured.name}</h2>
        {featured.description && <p>{featured.description}</p>}
        <Link to={'/title/' + first.source.id + '/' + featured.type + '/' + encodeURIComponent(featured.id)}>
          Ver detalhes <span aria-hidden="true">→</span>
        </Link>
      </div>
    </section>}
    <div className="yf-catalog-toolbar">
      <div><span className="yf-kicker">ESCOLHA O QUE ASSISTIR</span><h2>Explore sua biblioteca</h2></div>
      <div className="yf-catalog-filters" role="group" aria-label="Filtrar títulos">
        {([['all', 'Tudo'], ['movie', 'Filmes'], ['series', 'Séries']] as const).map(([key, label]) =>
          <button type="button" key={key} aria-pressed={kind === key} className={kind === key ? 'is-active' : ''}
            onClick={() => startTransition(() => setKind(key))}>{label}</button>)}
      </div>
    </div>
    <ViewTransition name="yf-catalog-filter">
      <div className="yf-catalog-shelves" key={kind}>
        {visible.map(s => <MediaShelf key={s.source.id + ':' + s.catalog.type + ':' + s.catalog.id} shelf={s} />)}
        {!visible.some(s => s.items.length > 0) && <div className="yf-catalog-empty" role="status">
          <strong>Nada por aqui ainda.</strong>
          <p>{kind === 'all' ? 'As fontes não retornaram títulos neste momento.' :
            'Nenhum título desta categoria foi retornado pelos catálogos carregados.'}</p>
        </div>}
      </div>
    </ViewTransition>
    {shelves.some(s => s.failed) && <p className="yf-catalog-note">Alguns catálogos estão temporariamente indisponíveis.</p>}
    <p className="yf-catalog-note">O catálogo reúne metadados de terceiros. A disponibilidade de reprodução depende das fontes e da compatibilidade com seu dispositivo.</p>
  </div>
}
