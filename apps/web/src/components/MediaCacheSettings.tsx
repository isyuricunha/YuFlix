import { useEffect, useState } from 'react'
import { api } from '../api'

export type MediaCachePolicy = {
  mode: 'automatic' | 'manual'
  storage: 'ram' | 'disk'
  maxMiB: number
  maxConcurrent: number
  segmentSeconds: number
  idleSeconds: number
  cleanupGraceSeconds: number
}
type Response = {
  policy: MediaCachePolicy
  defaults: MediaCachePolicy
  active: boolean
  status: string
}
const safeDefaults: MediaCachePolicy = {
  mode: 'automatic', storage: 'ram', maxMiB: 128,
  maxConcurrent: 1, segmentSeconds: 3, idleSeconds: 90, cleanupGraceSeconds: 15,
}

export default function MediaCacheSettings() {
  const [value, setValue] = useState<MediaCachePolicy>(safeDefaults)
  const [defaults, setDefaults] = useState<MediaCachePolicy>(safeDefaults)
  const [status, setStatus] = useState('')
  const [active, setActive] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    let alive = true
    api<Response>('/admin/media/cache').then(data => {
      if (!alive) return
      setValue(data.policy); setDefaults(data.defaults)
      setActive(data.active); setStatus(data.status)
    }).catch(() => { if (alive) setError('Não foi possível carregar as configurações do Media Engine.') })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [])

  function update<K extends keyof MediaCachePolicy>(key: K, next: MediaCachePolicy[K]) {
    setValue(current => ({ ...current, [key]: next }))
    setNotice('')
  }

  async function save() {
    if (saving) return
    setSaving(true); setNotice(''); setError('')
    try {
      const response = await api<Response>('/admin/media/cache', {
        method: 'PUT', body: JSON.stringify({ policy: value }),
      })
      setValue(response.policy); setActive(response.active); setStatus(response.status)
      setNotice('Configuração salva no banco.')
    } catch {
      setError('Não foi possível salvar. Confira os limites de cada campo.')
    } finally { setSaving(false) }
  }

  const manual = value.mode === 'manual'
  return <section className="yf-admin__preferences yf-media-cache" aria-labelledby="media-cache-title">
    <h2 id="media-cache-title">Transcode e cache HLS</h2>
    <p className="yf-admin__muted">
      Defina a política de recursos do Media Engine. RAM usa armazenamento temporário; disco usa o diretório exclusivo do worker.
      Os arquivos temporários são descartados após o término ou expiração da sessão.
    </p>
    {loading ? <p role="status">Carregando configuração…</p> : <>
      <p className="yf-media-cache__status" role="status">
        {active ? 'Worker HLS ativo.' : status || 'Worker HLS ainda não ativo.'}
      </p>
      <div className="yf-admin__setting">
        <div><h3>Gerenciamento</h3><p>Automático protege o LAB priorizando RAM e uma conversão de cada vez.</p></div>
        <div className="yf-admin__setting-action">
          <label className="yf-sr-only" htmlFor="media-mode">Gerenciamento de cache</label>
          <select id="media-mode" value={value.mode} onChange={e => update('mode', e.target.value as MediaCachePolicy['mode'])}>
            <option value="automatic">Automático (recomendado)</option>
            <option value="manual">Personalizado</option>
          </select>
        </div>
      </div>
      <div className="yf-admin__setting">
        <div><h3>Local do cache</h3><p>RAM evita gravações no SSD. Disco utiliza armazenamento persistente, com limpeza automática.</p></div>
        <div className="yf-admin__setting-action">
          <label className="yf-sr-only" htmlFor="media-storage">Local do cache de vídeo</label>
          <select id="media-storage" disabled={!manual} value={value.storage}
            onChange={e => update('storage', e.target.value as MediaCachePolicy['storage'])}>
            <option value="ram">RAM / tmpfs</option>
            <option value="disk">Disco</option>
          </select>
        </div>
      </div>
      {([
        ['maxMiB', 'Limite máximo de cache', 'MiB', 64, 4096, 64],
        ['maxConcurrent', 'Conversões simultâneas', 'sessões', 1, 4, 1],
        ['segmentSeconds', 'Duração dos segmentos', 'segundos', 2, 10, 1],
        ['idleSeconds', 'Tempo sem atividade', 'segundos', 30, 600, 10],
        ['cleanupGraceSeconds', 'Espera antes da limpeza', 'segundos', 0, 180, 5],
      ] as const).map(([key, label, unit, min, max, step]) =>
        <div className="yf-admin__setting" key={key}>
          <div><h3>{label}</h3><p>{unit}. Intervalo permitido: {min} a {max}.</p></div>
          <div className="yf-admin__setting-action">
            <label htmlFor={'media-'+key} className="yf-sr-only">{label}</label>
            <input id={'media-'+key} type="number" disabled={!manual}
              min={min} max={max} step={step} value={value[key]}
              onChange={e => update(key, Number(e.target.value))} />
          </div>
        </div>
      )}
      <div className="yf-media-cache__actions">
        <button type="button" disabled={saving} onClick={() => {setValue({...defaults});setNotice('Valores padrão restaurados no formulário. Clique em Salvar para aplicar.') }}>
          Restaurar padrões
        </button>
        <button type="button" disabled={saving} onClick={() => void save()}>
          {saving ? 'Salvando…' : 'Salvar configuração'}
        </button>
      </div>
      {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
      {error && <p role="alert" className="yf-alert">{error}</p>}
    </>}
  </section>
}
