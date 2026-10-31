import { useEffect, useState, type FormEvent } from 'react'
import { api, type StreamAddon } from '../api'

export default function StreamSources() {
  const [sources, setSources] = useState<StreamAddon[]>([])
  const [manifestURL, setManifestURL] = useState('')
  const [pending, setPending] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  useEffect(() => {
    let active = true
    api<{sources:StreamAddon[]}>('/media/addons').then(response => {
      if(active)setSources(response.sources)
    }).catch(() => {if(active)setError('Não foi possível carregar os provedores.')})
      .finally(()=>{if(active)setLoading(false)})
    return ()=>{active=false}
  }, [])
  async function add(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if(pending)return
    setError('');setNotice('');setPending(true)
    try {
      const response = await api<{source:StreamAddon}>('/admin/media/addons',{
        method:'POST',body:JSON.stringify({manifestUrl:manifestURL.trim()}),
      })
      setSources(prev=>[...prev,response.source])
      setManifestURL('')
      setNotice('Provedor cadastrado. A descoberta de fontes estará disponível nos títulos.')
    } catch {
      setError('Não foi possível cadastrar. Use um manifesto Stremio público HTTPS com recurso de stream.')
    } finally {setPending(false)}
  }
  async function remove(source: StreamAddon) {
    if(pending || !window.confirm('Remover o provedor '+source.name+'?'))return
    setError('');setNotice('');setPending(true)
    try {
      await api<void>('/admin/media/addons/'+source.id,{method:'DELETE'})
      setSources(prev=>prev.filter(s=>s.id!==source.id))
      setNotice('Provedor removido da instalação.')
    } catch {setError('Não foi possível remover o provedor.')}
    finally {setPending(false)}
  }
  return <section className="yf-admin__catalogs" aria-labelledby="streams-title">
    <h2 id="streams-title">Provedores de vídeo</h2>
    <p className="yf-admin__muted">Fontes compartilhadas pelos usuários. Cada pessoa mantém a própria sessão. Apenas MP4 público por HTTPS com suporte a Range pode ser reproduzido nesta etapa.</p>
    {loading && <p role="status" className="yf-admin__muted">Carregando provedores...</p>}
    {!loading && <div className="yf-admin__source-list">
      {sources.map(source=><div className="yf-admin__source" key={source.id}>
        <div><strong>{source.name}</strong><small>Addon de reprodução Stremio</small></div>
        <button disabled={pending} type="button" onClick={()=>remove(source)}>Remover</button>
      </div>)}
      {!sources.length && <p className="yf-admin__muted">Nenhum provedor de vídeo cadastrado.</p>}
    </div>}
    <div className="yf-admin__source-create">
      <h3>Adicionar addon de reprodução</h3>
      <form className="yf-admin__form" onSubmit={add}>
        <label htmlFor="stream-manifest">Manifesto do addon</label>
        <input id="stream-manifest" type="url" autoComplete="url" required value={manifestURL}
          disabled={pending} placeholder="https://addon.example.com/manifest.json"
          onChange={event=>setManifestURL(event.target.value)}/>
        <button className="yf-admin__primary" type="submit" disabled={pending}>{pending?'Validando...':'Adicionar provedor'}</button>
      </form>
      <p className="yf-admin__muted">Não cole tokens ou URLs privadas. Addons autenticados, HLS, MKV e proxies com headers especiais ainda não são suportados.</p>
    </div>
    {error && <p role="alert" className="yf-alert">{error}</p>}
    {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
  </section>
}
