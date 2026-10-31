import { useEffect, useState } from 'react'

type SystemInfo = {
  name: string
  slogan: string
  apiVersion: string
  phase: 'foundation'
  playbackReady: boolean
}

type State =
  | { kind: 'loading' }
  | { kind: 'connected'; data: SystemInfo }
  | { kind: 'unavailable' }

export default function App() {
  const [system, setSystem] = useState<State>({ kind: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetch('/api/v1/system/info', { signal: controller.signal, cache: 'no-store' })
      .then((response) => {
        if (!response.ok) throw new Error('Core unavailable')
        return response.json() as Promise<SystemInfo>
      })
      .then((data) => {
        if (!controller.signal.aborted) setSystem({ kind: 'connected', data })
      })
      .catch(() => {
        if (!controller.signal.aborted) setSystem({ kind: 'unavailable' })
      })
    return () => controller.abort()
  }, [])

  const available = system.kind === 'connected'
  return (
    <div className="shell">
      <header className="masthead" aria-label="Barra principal">
        <a className="brand" href="/" aria-label="YuFlix, início">
          <span className="brand-symbol" aria-hidden="true">Y</span>
          <span>YuFlix</span>
        </a>
        <span className="build-stage">FUNDAÇÃO / 001</span>
      </header>

      <main>
        <section className="hero" aria-labelledby="hero-heading">
          <div className="hero-label"><span className="accent-line" /> UMA NOVA FORMA DE ASSISTIR</div>
          <h1 id="hero-heading">
            I Watch<br />
            <span>What I Want.</span>
          </h1>
          <p className="hero-description">
            Seu catálogo. Suas fontes. Seu ritmo.
            Uma experiência de streaming independente, construída para reproduzir bem.
          </p>
          <div className="hero-footnote">
            <span className="line" />
            <span>YuFlix / Em desenvolvimento</span>
          </div>
          <div className="hero-mark" aria-hidden="true">Y</div>
        </section>

        <section className="status-section" aria-labelledby="system-heading">
          <div className="section-heading">
            <div>
              <span className="eyebrow">01 / STATUS DA PLATAFORMA</span>
              <h2 id="system-heading">Sistema</h2>
            </div>
            <span className={'connection-state ' + (available ? 'online' : '')} role="status">
              <span className="indicator" />
              {available ? 'CORE CONECTADO' : system.kind === 'loading' ? 'CONECTANDO' : 'INDISPONÍVEL'}
            </span>
          </div>
          <div className="status-details">
            <div className="detail">
              <span>API</span>
              <strong>{available ? system.data.apiVersion.toUpperCase() : '...'}</strong>
            </div>
            <div className="detail">
              <span>Fundação</span>
              <strong>{available ? 'Operacional' : system.kind === 'loading' ? 'Verificando' : 'Sem conexão'}</strong>
            </div>
            <div className="detail">
              <span>Reprodução</span>
              <strong>{available && system.data.playbackReady ? 'Disponível' : 'Em desenvolvimento'}</strong>
            </div>
          </div>
          <p className="status-note">
            A estrutura inicial já permite verificar a comunicação entre o navegador e o Core.
            O player e os catálogos só serão disponibilizados após testes reais.
          </p>
        </section>
      </main>
      <footer className="footer">
        <span>YUFLIX</span>
        <span>I WATCH WHAT I WANT.</span>
        <a href="https://github.com/isyuricunha/YuFlix" rel="noopener noreferrer" target="_blank">Código-fonte</a>
      </footer>
    </div>
  )
}
