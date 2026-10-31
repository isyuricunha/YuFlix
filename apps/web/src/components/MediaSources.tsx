import { useEffect, useRef, useState } from 'react'
import { api, type PlaybackSession } from '../api'
import PlayerOverlay from './PlayerOverlay'

type Props = {
  type: 'movie' | 'series'
  videoId: string
  isAdmin: boolean
  contentTitle?: string
  autoStart?: boolean
  onNext?: () => void
}

export default function MediaSources({ type, videoId, autoStart = false, onNext, contentTitle }: Props) {
  const [session, setSession] = useState<PlaybackSession | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')
  const request = useRef<AbortController | null>(null)
  const attempt = useRef(0)

  useEffect(() => {
    setSession(null)
    setMessage('')
    setLoading(false)
    attempt.current = 0
    if (autoStart) void play(0)
    return () => {
      request.current?.abort()
      request.current = null
    }
    // A new title/episode starts a new playback lifecycle.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type, videoId])

  async function play(nextAttempt: number) {
    if (nextAttempt > 7) {
      setSession(null)
      setLoading(false)
      setMessage('Não foi possível reproduzir este título com as fontes disponíveis.')
      return
    }
    request.current?.abort()
    const controller = new AbortController()
    request.current = controller
    attempt.current = nextAttempt
    setLoading(true)
    setMessage(nextAttempt ? 'Tentando outra fonte disponível…' : 'Preparando reprodução…')
    setSession(null)
    try {
      const data = await api<PlaybackSession>('/media/auto', {
        method: 'POST',
        body: JSON.stringify({ type, id: videoId, attempt: nextAttempt }),
        signal: controller.signal,
      })
      if (!data.path.startsWith('/api/v1/media/play/')) throw new Error('invalid media ticket')
      if (!controller.signal.aborted) {
        attempt.current = data.nextAttempt ?? nextAttempt + 1
        setSession({...data,name:contentTitle || data.name})
        setMessage('')
      }
    } catch {
      if (!controller.signal.aborted) {
        setMessage('Nenhuma fonte compatível está disponível para reprodução neste momento.')
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false)
    }
  }

  return <section className="yf-playback yf-playback--automatic" aria-labelledby="playback-title">
    <div className="yf-playback__heading">
      <div><span className="yf-kicker">PRONTO PARA ASSISTIR</span><h2 id="playback-title">Reprodução</h2></div>
      <button className="yf-playback__play" type="button" disabled={loading}
        onClick={() => void play(0)}>
        <span aria-hidden="true">▶</span> {loading ? 'Preparando…' : 'Assistir agora'}
      </button>
    </div>
    {loading && <p className="yf-playback__loading" role="status">{message}</p>}
    {!loading && message && <div className="yf-playback__notice" role="status">
      <p>{message}</p>
      <button type="button" onClick={() => void play(0)}>Tentar novamente</button>
    </div>}
    {session && <PlayerOverlay session={session} onClose={() => setSession(null)}
      onEnded={onNext}
      onPlaybackError={() => void play(attempt.current)} />}
  </section>
}
