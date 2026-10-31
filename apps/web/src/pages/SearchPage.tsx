import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api, type CatalogItem } from '../api'

type Result = CatalogItem & { sourceId: string; catalog: string }
type SearchResponse = {items:Result[]; scope:'cached_catalogs';query:string;note:string}

export default function SearchPage() {
  const [params,setParams] = useSearchParams()
  const query = params.get('q') || ''
  const [input,setInput] = useState(query)
  const [results,setResults] = useState<Result[]>([])
  const [loading,setLoading] = useState(false)
  const [error,setError] = useState('')

  useEffect(()=>{
    const phrase = input.trim()
    if (phrase.length < 2) {setResults([]);setLoading(false);setError('');return}
    const controller = new AbortController()
    setLoading(true);setError('')
    const timer=window.setTimeout(()=>{
      api<SearchResponse>('/catalog/search?q='+encodeURIComponent(phrase),{signal:controller.signal})
        .then(data=>{if(!controller.signal.aborted)setResults(data.items)})
        .catch(()=>{if(!controller.signal.aborted)setError('Não foi possível consultar o catálogo local.')})
        .finally(()=>{if(!controller.signal.aborted)setLoading(false)})
    },280)
    return ()=>{controller.abort();window.clearTimeout(timer)}
  },[input])

  return <section className="yf-search-page" aria-labelledby="search-title">
    <div className="yf-search-page__intro">
      <span className="yf-kicker">DESCOBRIR NO YUFLIX</span>
      <h1 id="search-title">Encontre sua próxima história.</h1>
      <p>Busque pelos títulos já disponíveis no catálogo local, sem esperar uma nova consulta a cada provedor.</p>
    </div>
    <form className="yf-search-page__form" onSubmit={e=>{
      e.preventDefault()
      setParams(input.trim()?{q:input.trim()}:{} )
    }}>
      <label htmlFor="search-titles" className="yf-sr-only">Buscar filmes, séries e animes</label>
      <input id="search-titles" type="search" autoFocus autoComplete="off"
        placeholder="Filmes, séries, animes…" maxLength={64} value={input}
        onChange={e=>setInput(e.target.value)} />
      <button type="submit">Buscar</button>
    </form>
    <div className="yf-search-page__results">
      <div className="yf-search-page__results-heading">
        <h2>{loading ? 'Procurando…' : input.trim().length<2 ? 'Comece digitando' :
          results.length ? results.length + ' resultados no catálogo local' : 'Nenhum resultado no cache'}</h2>
        <span>Catálogos armazenados</span>
      </div>
      {error && <p role="alert" className="yf-alert">{error}</p>}
      {!loading && input.trim().length>1 && !results.length && !error &&
        <p className="yf-catalog-note">O título pode existir em um provedor ainda não carregado. Esta busca consulta somente metadados armazenados, não a disponibilidade de streams.</p>}
      {results.length>0 && <div className="yf-search-page__grid">
        {results.map(item=><Link className="yf-media-card" key={item.sourceId+':'+item.type+':'+item.id}
          to={'/title/'+item.sourceId+'/'+item.type+'/'+encodeURIComponent(item.id)} aria-label={'Ver detalhes de '+item.name}>
          <div className="yf-media-card__image">
            {item.poster ? <img src={item.poster} loading="lazy" alt="" referrerPolicy="no-referrer" /> :
              <span aria-hidden="true">{item.name.charAt(0)}</span>}
          </div>
          <strong>{item.name}</strong>
          <small>{item.releaseInfo || (item.type==='movie'?'Filme':'Série')}</small>
        </Link>)}
      </div>}
    </div>
  </section>
}
