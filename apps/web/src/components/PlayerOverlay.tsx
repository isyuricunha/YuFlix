import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import type { PlaybackSession } from '../api'

export default function PlayerOverlay({ session, onClose, onEnded, onPlaybackError }: { session: PlaybackSession; onClose: () => void; onEnded?: () => void; onPlaybackError?: () => void }) {
  const player = useRef<HTMLVideoElement>(null)
  const dialog = useRef<HTMLDivElement>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  const [state, setState] = useState<'loading' | 'ready' | 'waiting' | 'ended' | 'error'>('loading')
  const [closing, setClosing] = useState(false)

  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    closeButton.current?.focus()
    return () => {
      document.body.style.overflow = overflow
      previous?.focus()
    }
  }, [])

  useEffect(() => {
    if (!closing) return
    const timer = window.setTimeout(onClose, 180)
    return () => window.clearTimeout(timer)
  }, [closing, onClose])

  function onDialogKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') {
      event.preventDefault()
      setClosing(true)
      return
    }
    if (event.key !== 'Tab') return
    const elements = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled), video[tabindex]') ?? [])
    const first = elements[0]
    const last = elements[elements.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last?.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first?.focus()
    }
  }

  async function enterFullscreen() {
    const element = player.current
    if (!element) return
    try {
      if (document.fullscreenElement) await document.exitFullscreen()
      else if (element.requestFullscreen) await element.requestFullscreen()
      else element.focus()
    } catch {
      element.focus()
    }
  }

  const status = state === 'loading' ? 'Preparando reprodução…'
    : state === 'waiting' ? 'Aguardando dados do vídeo…'
    : state === 'ended' ? 'Reprodução finalizada.'
    : state === 'error' ? 'Esta fonte não pôde ser reproduzida neste dispositivo. Feche e escolha outra fonte.'
    : ''

  return createPortal(<div className={'yf-player-overlay' + (closing ? ' is-closing' : '')}
    role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) setClosing(true) }}>
    <div className="yf-player" ref={dialog} role="dialog" aria-modal="true" aria-label={'Reprodução: ' + session.name}
      onKeyDown={onDialogKeyDown}>
      <div className="yf-player__topbar">
        <div className="yf-player__identity">
          <span className="yf-kicker">REPRODUZINDO NO YUFLIX</span>
          <h2>{session.name}</h2>
          <p>{session.sourceName || 'Reprodução automática'} <span aria-hidden="true">·</span> Direct Play</p>
        </div>
        <div className="yf-player__actions">
          <button type="button" onClick={() => void enterFullscreen()} aria-label="Tela cheia" className="yf-player__fullscreen">Tela cheia</button>
          <button type="button" onClick={() => setClosing(true)} ref={closeButton} aria-label="Fechar player" className="yf-player__close">
            <span aria-hidden="true">×</span><span className="yf-sr-only">Fechar player</span>
          </button>
        </div>
      </div>
      <div className="yf-player__stage">
        <video ref={player} autoPlay controls playsInline preload="metadata" tabIndex={0} src={session.path}
          aria-label="Reprodutor YuFlix" onLoadedMetadata={() => setState('ready')}
          onPlaying={() => setState('ready')} onWaiting={() => setState('waiting')}
          onEnded={() => {setState('ended');onEnded?.()}} onError={() => {setState('error');onPlaybackError?.()}}/>
      </div>
      <div className="yf-player__bottom">
        <span role="status" aria-live="polite" className={state === 'error' ? 'yf-player__error' : ''}>{status || 'Use os controles do vídeo para reproduzir, pausar e avançar.'}</span>
        <span>Esc para fechar</span>
      </div>
    </div>
  </div>, document.body)
}
