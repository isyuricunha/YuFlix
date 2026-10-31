import { useEffect, useState, type Dispatch, type SetStateAction } from 'react'
import { Link } from 'react-router'
import { api, type CatalogSource, type CatalogDefinition, type CatalogItem } from '../api'

type Shelf = { source: CatalogSource; catalog: CatalogDefinition; items: CatalogItem[]; failed?: boolean }

function MediaShelf({ shelf }: { shelf: Shelf }) {
  if (!shelf.items.length) return null
  return <section className="yf-media-shelf" aria-label={shelf.catalog.name + ' de ' + shelf.source.name}>
    <div className="yf-media-shelf__heading">
      <h2>{shelf.catalog.type === 'movie' ? 'Filmes' : 'Séries'} · {shelf.catalog.name}</h2>
      <span>{shelf.source.name}</span>
    </div>
    <div className="yf-media-shelf__rail">
      {shelf.items.map(item =>
        <Link className="yf-media-card" key={item.id} to={'/title/'+shelf.source.id+'/'+item.type+'/'+encodeURIComponent(item.id)}
          aria-label={'Ver detalhes de ' + item.name}>
          <div className="yf-media-card__image">
            {item.poster && <img src={item.poster} alt="" loading="lazy" referrerPolicy="no-referrer" />}
            {!item.poster && <span aria-hidden="true">{item.name.slice(0,1)}</span>}
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
  useEffect(() => {
    let active = true
    async function load() {
      try {
        const response = await api<{ sources: CatalogSource[] }>('/catalog/sources')
        if (!active) return
        setSources(response.sources)
        onCatalogPresent(response.sources.length>0)
        const chosen = response.sources.filter(source => source.enabled)
          .flatMap(source => source.catalogs.filter(c => c.type === 'movie' || c.type === 'series')
            .slice(0,2).map(catalog => ({source,catalog}))).slice(0,4)
        if (!chosen.length) {setLoading(false);return}
        const all = await Promise.all(chosen.map(async item => {
          try {
            const params = '?type='+encodeURIComponent(item.catalog.type)+'&catalog='+encodeURIComponent(item.catalog.id)
            const data = await api<{ items: CatalogItem[] }>('/catalog/sources/'+item.source.id+'/items'+params)
            return { ...item, items: data.items }
          } catch { return { ...item, items: [], failed: true } }
        }))
        if (active) {setShelves(all);setLoading(false)}
      } catch {
        if (active) {setError('Não foi possível carregar o catálogo.');setLoading(false)}
      }
    }
    void load()
    return () => {active=false}
  }, [onCatalogPresent])
  const first = shelves.find(s => s.items.some(i => i.background))
  const featured = first?.items.find(i => i.background)
  if (loading) return <section className="yf-library yf-library--loading" role="status">Carregando sua biblioteca...</section>
  if (error) return <section className="yf-library" role="alert">{error}</section>
  if (!sources.length) return <section className="yf-library" aria-labelledby="library-title">
    <div className="yf-library__heading"><div><span className="yf-kicker">SUA BIBLIOTECA</span>
      <h2 id="library-title">Tudo em um lugar.</h2></div><span className="yf-library__phase">Em preparação</span></div>
    <div className="yf-library__empty"><div className="yf-library__glyph" aria-hidden="true"><span>▶</span></div>
      <div className="yf-library__message"><h3>Conecte seu primeiro catálogo.</h3>
        <p>Os títulos serão exibidos aqui quando um administrador cadastrar uma fonte. Catálogo não significa reprodução disponível.</p></div>
      {isAdmin && <Link to="/admin">Configurar fontes <span aria-hidden="true">→</span></Link>}
    </div>
  </section>
  return <div className="yf-catalog-browse">
    {featured && first && <section className="yf-catalog-featured" aria-labelledby="featured-title">
      {featured.background && <img className="yf-catalog-featured__backdrop" src={featured.background} alt="" referrerPolicy="no-referrer" />}
      <div className="yf-catalog-featured__content">
        <span className="yf-kicker">EM DESTAQUE NO CATÁLOGO</span>
        <h2 id="featured-title">{featured.name}</h2>
        {featured.description && <p>{featured.description}</p>}
        <Link to={'/title/'+first.source.id+'/'+featured.type+'/'+encodeURIComponent(featured.id)}>Ver detalhes <span aria-hidden="true">→</span></Link>
      </div>
    </section>}
    {shelves.map(s => <MediaShelf key={s.source.id+':'+s.catalog.type+':'+s.catalog.id} shelf={s}/>)}
    {!shelves.some(s => s.items.length>0) && <div className="yf-library">
      <p>As fontes não retornaram títulos neste momento. Tente novamente mais tarde.</p>
    </div>}
    {shelves.some(s=>s.failed) && <p className="yf-catalog-note">Alguns catálogos estão temporariamente indisponíveis.</p>}
    <p className="yf-catalog-note">Títulos exibidos são metadados de terceiros. A reprodução ainda não está disponível.</p>
  </div>
}
