import { useEffect, useState, type FormEvent } from 'react'
import { api, type CatalogSource } from '../api'

export default function CatalogSources() {
  const [sources, setSources] = useState<CatalogSource[]>([])
  const [url, setURL] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    let live = true
    api<{ sources: CatalogSource[] }>('/catalog/sources').then(data => {
      if (live) setSources(data.sources)
    }).catch(() => {if(live)setError('Não foi possível carregar as fontes.')})
      .finally(() => {if(live)setLoading(false)})
    return () => {live=false}
  }, [])

  async function add(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    setBusy(true);setError('');setNotice('')
    try {
      const result = await api<{source: CatalogSource}>('/admin/catalog/sources',{
        method:'POST',body:JSON.stringify({manifestUrl:url.trim()}),
      })
      setSources(old => [...old,result.source])
      setURL('')
      setNotice('Catálogo cadastrado. Os títulos já podem aparecer na página inicial.')
    } catch {
      setError('Não foi possível adicionar a fonte. Confira se é um manifesto HTTPS público compatível.')
    } finally {setBusy(false)}
  }

  async function remove(source: CatalogSource) {
    if (!window.confirm('Remover '+source.name+' do catálogo compartilhado?')) return
    setBusy(true);setError('');setNotice('')
    try {
      await api<void>('/admin/catalog/sources/'+source.id,{method:'DELETE'})
      setSources(old=>old.filter(item=>item.id!==source.id))
      setNotice('Fonte removida.')
    } catch {setError('Não foi possível remover essa fonte.')}
    finally {setBusy(false)}
  }

  return <section className="yf-admin__catalogs" aria-labelledby="catalog-title">
    <h2 id="catalog-title">Fontes compartilhadas</h2>
    <p className="yf-admin__muted">Adicione catálogos do protocolo Stremio. Todos os usuários verão os mesmos títulos, mas mantêm sessões e histórico individuais.</p>
    {loading && <p role="status" className="yf-admin__muted">Carregando fontes...</p>}
    {!loading && <div className="yf-admin__source-list">
      {sources.map(source => <div className="yf-admin__source" key={source.id}>
        <div><strong>{source.name}</strong><small>{source.catalogs.length} catálogos de filmes e séries</small></div>
        <button disabled={busy} type="button" onClick={() => remove(source)}>Remover</button>
      </div>)}
      {!sources.length && <p className="yf-admin__muted">Nenhuma fonte cadastrada.</p>}
    </div>}
    <div className="yf-admin__source-create">
      <h3>Adicionar catálogo Stremio</h3>
      <form onSubmit={add} className="yf-admin__form">
        <label htmlFor="addon-url">Endereço do manifesto</label>
        <input id="addon-url" type="url" required autoComplete="url" value={url} disabled={busy}
          placeholder="https://v3-cinemeta.strem.io/manifest.json" onChange={e=>setURL(e.target.value)} />
        <button type="submit" className="yf-admin__primary" disabled={busy}>{busy?'Validando manifesto...':'Adicionar fonte'}</button>
      </form>
      <p className="yf-admin__muted">Por segurança, esta etapa aceita apenas URLs públicas HTTPS com configurações no caminho, sem query string. Addons privados e IPTV serão tratados em uma integração separada.</p>
    </div>
    {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
    {error && <p role="alert" className="yf-alert">{error}</p>}
  </section>
}
