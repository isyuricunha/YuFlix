import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { api, type MediaCandidate, type PlaybackSession, type ProviderStatus } from '../api'
import PlayerOverlay from './PlayerOverlay'

export default function MediaSources({ type, videoId, isAdmin }: {
  type: 'movie' | 'series'; videoId: string; isAdmin: boolean
}) {
  const [candidates, setCandidates] = useState<MediaCandidate[]>([])
  const [providers, setProviders] = useState(0)
  const [statuses, setStatuses] = useState<ProviderStatus[]>([])
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState<string | null>(null)
  const [session, setSession] = useState<PlaybackSession | null>(null)
  const [message, setMessage] = useState('')
  const request = useRef(0)
  const sessionRequest = useRef(0)

  useEffect(() => {
    request.current++
    sessionRequest.current++
    setCandidates([])
    setProviders(0)
    setStatuses([])
    setSession(null)
    setChecking(null)
    setLoading(false)
    setMessage('')
    return () => { request.current++; sessionRequest.current++ }
  }, [type, videoId])

  async function discover() {
    const current = ++request.current
    sessionRequest.current++
    setLoading(true); setMessage(''); setSession(null); setStatuses([]); setCandidates([])
    try {
      const params = '?type=' + encodeURIComponent(type) + '&id=' + encodeURIComponent(videoId)
      const data = await api<{ streams: MediaCandidate[], providersChecked: number, providersFailed: number, providerStatuses?: ProviderStatus[] }>('/media/streams' + params)
      if (current !== request.current) return
      setCandidates(data.streams)
      setProviders(data.providersChecked)
      setStatuses(data.providerStatuses ?? [])
      if (!data.streams.length) {
        setMessage(data.providersChecked === 0 ? 'Nenhum provedor de vídeo cadastrado.' :
          'Nenhuma fonte compatível foi encontrada. Confira o resultado dos provedores abaixo.')
      }
    } catch {
      if (current === request.current) setMessage('Não foi possível consultar as fontes de vídeo.')
    } finally {
      if (current === request.current) setLoading(false)
    }
  }

  async function play(stream: MediaCandidate) {
    if (checking || loading) return
    const current = ++sessionRequest.current
    setChecking(stream.sourceId + ':' + stream.index); setMessage(''); setSession(null)
    try {
      const data = await api<PlaybackSession>('/media/sessions', {
        method: 'POST', body: JSON.stringify({ sourceId: stream.sourceId, type, id: videoId, index: stream.index, candidateKey: stream.key }),
      })
      if (!data.path.startsWith('/api/v1/media/play/')) throw new Error('invalid media route')
      if (current === sessionRequest.current) setSession(data)
    } catch {
      if (current === sessionRequest.current) setMessage('Esta fonte não passou na verificação de MP4. Tente outra.')
    } finally {
      if (current === sessionRequest.current) setChecking(null)
    }
  }

  return <section className="yf-playback" aria-labelledby="playback-title">
    <div className="yf-playback__heading">
      <div><span className="yf-kicker">REPRODUÇÃO</span><h2 id="playback-title">Fontes de vídeo</h2></div>
      <button type="button" disabled={loading || !!checking} onClick={() => void discover()}>
        {loading ? 'Procurando…' : candidates.length ? 'Atualizar fontes' : 'Buscar fontes'}
      </button>
    </div>
    <p className="yf-playback__description">Escolha uma fonte e assista no player. Nesta etapa, o Direct Play suporta MP4 compatível, com busca por posição.</p>
    {loading && <p className="yf-playback__loading" role="status">Consultando os provedores disponíveis…</p>}
    {message && <p className="yf-playback__notice" role="status">{message}
      {providers === 0 && isAdmin && <span> <Link to="/admin?section=streams">Configurar provedores</Link></span>}
    </p>}
    {!!statuses.length && statuses.some(source => source.state !== 'ok') && <details className="yf-playback__diagnostics" open={!candidates.length}>
      <summary>Detalhes dos provedores <span>({statuses.filter(source => source.state !== 'ok').length})</span></summary>
      <div>{statuses.filter(source => source.state !== 'ok').map((source, index) => <p className="yf-playback__notice" key={source.name + index} role="status">
        <strong>{source.name}:</strong>{' '}
        {source.state === 'empty' ? 'Nenhum vídeo retornado.' :
          source.state === 'unsupported' ? source.received + ' fonte(s) retornada(s), mas nenhuma compatível com Direct Play MP4.' :
          source.reason === 'access_denied' ? 'Acesso recusado pelo provedor (HTTP ' + (source.httpStatus || 403) + ').' :
          source.reason === 'not_found' ? 'Conteúdo não encontrado pelo addon (HTTP 404).' :
          source.reason === 'rate_limited' ? 'Provedor limitou temporariamente as consultas (HTTP 429).' :
          source.reason === 'timeout' ? 'Tempo limite da consulta atingido.' :
          source.reason === 'unsafe_address' ? 'O destino foi bloqueado pela política de segurança.' :
          source.reason === 'invalid_response' ? 'Resposta inválida ou incompatível com o protocolo Stremio.' :
          source.reason === 'upstream_unavailable' ? 'Servidor do addon indisponível (HTTP ' + (source.httpStatus || 500) + ').' :
          source.httpStatus ? 'Erro HTTP ' + source.httpStatus + ' retornado pelo addon.' :
            'Não foi possível conectar ao addon.'}
      </p>)}</div>
    </details>}
    {!!candidates.length && <div className="yf-playback__sources" aria-label="Fontes disponíveis">
      <div className="yf-playback__sources-heading"><span>FONTES ENCONTRADAS</span><span>{candidates.length}</span></div>
      {candidates.map((item, index) => <div className="yf-playback__source" key={item.sourceId + ':' + item.index}>
        <span className="yf-playback__source-index" aria-hidden="true">{String(index + 1).padStart(2, '0')}</span>
        <div className="yf-playback__source-details"><strong>{item.name}</strong><span>{item.sourceName}{item.title ? ' · ' + item.title : ''}</span></div>
        <button type="button" disabled={!!checking || loading} onClick={() => void play(item)}>
          {checking === item.sourceId + ':' + item.index ? 'Validando…' : 'Testar Direct Play'}
        </button>
      </div>)}
    </div>}
    {session && <PlayerOverlay session={session} onClose={() => setSession(null)} />}
  </section>
}
