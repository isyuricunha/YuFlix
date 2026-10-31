import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { api, type CatalogDetail } from '../api'
import Brand from '../components/Brand'

export default function TitlePage() {
  const { sourceId, type, id } = useParams()
  const [item, setItem] = useState<CatalogDetail | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    if (!sourceId || !type || !id) return
    const query = '?type='+encodeURIComponent(type)+'&id='+encodeURIComponent(id)
    api<{meta: CatalogDetail}>('/catalog/sources/'+sourceId+'/meta'+query).then(data => {
      if (active) setItem(data.meta)
    }).catch(() => {if(active)setError('Não foi possível carregar os detalhes deste título.')})
    return () => {active=false}
  }, [sourceId,type,id])
  return <div className="yf-title-page">
    <header className="yf-title-page__header"><Brand compact /><Link to="/">Voltar à biblioteca</Link></header>
    {error && <main className="yf-title-page__error" role="alert">{error}</main>}
    {!error && !item && <main className="yf-title-page__loading" role="status">Carregando detalhes...</main>}
    {item && <main className="yf-title-page__main">
      {item.background && <img className="yf-title-page__background" src={item.background} alt="" referrerPolicy="no-referrer" />}
      <div className="yf-title-page__content">
        <span className="yf-kicker">{item.type === 'movie' ? 'FILME' : 'SÉRIE'}</span>
        <h1>{item.name}</h1>
        <div className="yf-title-page__meta">
          {item.releaseInfo && <span>{item.releaseInfo}</span>}
          {item.genres?.slice(0,3).map(genre => <span key={genre}>{genre}</span>)}
        </div>
        {item.description && <p>{item.description}</p>}
        <div className="yf-title-page__not-ready">
          <strong>Reprodução indisponível nesta etapa</strong>
          <span>O título foi encontrado no catálogo. Ainda não há uma fonte de vídeo validada para ele.</span>
        </div>
        {item.videos && item.videos.length > 0 && <section className="yf-title-page__episodes">
          <h2>Episódios</h2>
          <div>{item.videos.slice(0,30).map(ep => <div key={ep.id}>
            <strong>{ep.title || 'Episódio '+(ep.episode||'?')}</strong>
            <small>{ep.season ? 'T'+ep.season+' · ':''}E{ep.episode||'?'}</small>
          </div>)}</div>
        </section>}
      </div>
    </main>}
  </div>
}
