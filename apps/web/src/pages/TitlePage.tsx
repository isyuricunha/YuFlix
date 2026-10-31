import { startTransition, useEffect, useState, ViewTransition } from 'react'
import { Link, useParams } from 'react-router'
import { api, type CatalogDetail } from '../api'
import MediaSources from '../components/MediaSources'
import { useAuth } from '../auth/AuthContext'

export default function TitlePage() {
  const { sourceId, type, id } = useParams()
  const [item, setItem] = useState<CatalogDetail | null>(null)
  const [error, setError] = useState('')
  const [episodeId, setEpisodeId] = useState('')
  const [autoNext, setAutoNext] = useState(false)
  const { user } = useAuth()
  const isCollection = Boolean(item?.id.startsWith('ctmdb.'))

  useEffect(() => {
    let active = true
    if (!sourceId || !type || !id) return
    const query = '?type=' + encodeURIComponent(type) + '&id=' + encodeURIComponent(id)
    setItem(null)
    setError('')
    setEpisodeId('')
    setAutoNext(false)
    api<{ meta: CatalogDetail }>('/catalog/sources/' + sourceId + '/meta' + query)
      .then(data => { if (active) setItem(data.meta) })
      .catch(() => { if (active) setError('Não foi possível carregar os detalhes deste título.') })
    return () => { active = false }
  }, [sourceId, type, id])

  return <div className="yf-title-page">
    <div className="yf-title-page__breadcrumb"><Link to="/">Biblioteca</Link><span aria-hidden="true">/</span><span>Detalhes</span></div>
    {error && <div className="yf-title-page__error" role="alert">{error} <Link to="/">Voltar à biblioteca</Link></div>}
    {!error && !item && <div className="yf-title-page__loading" role="status">Carregando detalhes...</div>}
    {item && <div className="yf-title-page__main">
      {item.background && <img className="yf-title-page__background" src={item.background} alt="" referrerPolicy="no-referrer" />}
      <div className="yf-title-page__content">
        <span className="yf-kicker">{item.type === 'movie' ? 'FILME' : isCollection ? 'COLEÇÃO' : item.type === 'anime' ? 'ANIME' : 'SÉRIE'}</span>
        <h1>{item.name}</h1>
        <div className="yf-title-page__meta">
          {item.releaseInfo && <span>{item.releaseInfo}</span>}
          {item.genres?.slice(0, 3).map(genre => <span key={genre}>{genre}</span>)}
        </div>
        {item.description && <p>{item.description}</p>}
        {item.videos && item.videos.length > 0 && <section className="yf-title-page__episodes" aria-labelledby="episode-heading">
          <div className="yf-title-page__episodes-heading">
            <div><span className="yf-kicker">CONTINUAR EXPLORANDO</span><h2 id="episode-heading">{isCollection ? 'Filmes da coleção' : 'Episódios'}</h2></div>
            <span>{Math.min(item.videos.length, 30)} disponíveis no catálogo</span>
          </div>
          <div className="yf-title-page__episode-list">{item.videos.slice(0, 30).map(ep => <button type="button"
            className={episodeId === ep.id ? 'yf-episode is-selected' : 'yf-episode'}
            aria-pressed={episodeId === ep.id} key={ep.id} onClick={() => {setAutoNext(false);startTransition(() => setEpisodeId(ep.id))}}>
            <span className="yf-episode__number">{String(ep.episode || '?').padStart(2, '0')}</span>
            <span className="yf-episode__title"><strong>{ep.title || (isCollection ? 'Filme' : 'Episódio') + ' ' + (ep.episode || '?')}</strong>
              <small>{isCollection ? 'Filme da coleção' : (ep.season ? 'Temporada ' + ep.season + ' · ' : '') + 'Episódio ' + (ep.episode || '?')}</small>
            </span>
            <span className="yf-episode__arrow" aria-hidden="true">→</span>
          </button>)}</div>
        </section>}
        {isCollection ? (episodeId ? <MediaSources key={episodeId} type="movie" videoId={episodeId}
          contentTitle={item.videos?.find(video => video.id === episodeId)?.title || item.name}
          isAdmin={user?.role === 'admin'} /> :
          <p className="yf-catalog-note">Escolha um filme da coleção para encontrar uma reprodução compatível.</p>) :
        item.type === 'movie' ? <MediaSources type="movie" videoId={item.imdb_id?.match(/^tt[0-9]{7,10}$/) ? item.imdb_id : item.id} contentTitle={item.name} isAdmin={user?.role === 'admin'} /> :
          <ViewTransition name="yf-episode-media">
            <div key={episodeId || 'none'} className="yf-title-page__episode-media">
              {episodeId ? <MediaSources type="series" videoId={episodeId} contentTitle={item.name} isAdmin={user?.role === 'admin'}
                   autoStart={autoNext} onNext={() => {
                     const index = item.videos?.findIndex(ep => ep.id === episodeId) ?? -1
                     const next = item.videos?.[index + 1]
                     if (next) { setAutoNext(true); setEpisodeId(next.id) }
                   }} /> :
                <div className="yf-title-page__not-ready"><strong>Escolha um episódio</strong>
                  <span>Selecione um episódio acima para encontrar fontes de vídeo.</span></div>}
            </div>
          </ViewTransition>}
      </div>
    </div>}
  </div>
}
