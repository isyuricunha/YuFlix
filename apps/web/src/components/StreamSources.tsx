import { useEffect, useState, type FormEvent } from 'react'
import { APIError, api, type StreamAddon } from '../api'

export default function StreamSources() {
  const [sources, setSources] = useState<StreamAddon[]>([])
  const [manifestURL, setManifestURL] = useState('')
  const [egressBySource, setEgressBySource] = useState<Record<string,boolean>>({})
  const [policyDrafts,setPolicyDrafts] = useState<Record<string,{priority:number;timeoutMs:number}>>({})
  const [pending, setPending] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  useEffect(() => {
    let active = true
    Promise.all([
      api<{sources:StreamAddon[]}>('/admin/media/addons'),
      api<{availableForSources:Record<string,boolean>}>('/admin/network/egress'),
    ]).then(([response,network]) => {
      if(active){setSources(response.sources);setEgressBySource(network.availableForSources)}
    }).catch(() => {if(active)setError('Não foi possível carregar os provedores ou suas opções de rede.')})
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
    } catch (cause) {
      if (cause instanceof APIError) {
        setError(cause.status===422 ? 'O endereço foi aceito, mas o manifesto não respondeu ou não oferece streams compatíveis.' :
          cause.status===400 ? 'Endereço inválido. Cole a URL HTTPS completa do manifesto configurado.' :
          cause.status===409 ? 'Esse provedor já foi cadastrado ou o limite foi atingido.' :
          cause.message)
      } else {
        setError('Não foi possível contatar o YuFlix. Verifique a conexão com o servidor.')
      }
    } finally {setPending(false)}
  }
  async function updateRoute(source:StreamAddon,route:StreamAddon['networkRoute']){
    if(pending || route===source.networkRoute)return
    setPending(true);setError('');setNotice('')
    try{
      const result=await api<{source:StreamAddon}>('/admin/media/addons/'+source.id+'/route',{
        method:'PUT',body:JSON.stringify({route}),
      })
      setSources(previous=>previous.map(value=>value.id===source.id ? result.source : value))
      setNotice('Rota de rede atualizada. Sessões anteriores desta fonte foram invalidadas.')
    }catch{
      setError('Não foi possível alterar a rota. Confira se há um egress autorizado para essa fonte.')
    }finally{setPending(false)}
  }
  async function savePolicy(source: StreamAddon) {
    if (pending) return
    const value = policyDrafts[source.id] ?? {priority:source.priority ?? 500,timeoutMs:source.timeoutMs ?? 5000}
    if (!Number.isInteger(value.priority) || value.priority < 1 || value.priority > 1000 ||
        !Number.isInteger(value.timeoutMs) || value.timeoutMs < 1000 || value.timeoutMs > 10000) {
      setError('Prioridade de 1 a 1000 e timeout de 1 a 10 segundos.')
      return
    }
    setPending(true);setError('');setNotice('')
    try {
      const result=await api<{source:StreamAddon}>('/admin/media/addons/'+source.id+'/policy',{
        method:'PUT',body:JSON.stringify(value),
      })
      setSources(previous=>previous.map(item=>item.id===source.id ? result.source:item))
      setPolicyDrafts(previous=>{ const copy={...previous};delete copy[source.id];return copy })
      setNotice('Prioridade e tempo limite salvos.')
    }catch{setError('Não foi possível salvar a preferência desta fonte.')}
    finally{setPending(false)}
  }
  function draftFor(source:StreamAddon) {
    return policyDrafts[source.id] ?? {priority:source.priority ?? 500,timeoutMs:source.timeoutMs ?? 5000}
  }
  function changeDraft(source:StreamAddon, key:'priority'|'timeoutMs', value:number) {
    setPolicyDrafts(previous=>({...previous,[source.id]:{...draftFor(source),[key]:value}}))
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
    <p className="yf-admin__muted">Fontes compartilhadas pelos usuários. Cada pessoa mantém a própria sessão. A reprodução seleciona as fontes automaticamente, em ordem de prioridade, sem exibi-las aos usuários. Apenas MP4 HTTPS com Range é suportado nesta etapa.</p>
    <p className="yf-admin__muted">A saída direta é o padrão. O egress privado só aparece quando estiver autorizado para o provedor. A seleção é salva no PostgreSQL, sem expor credenciais ou endereços de infraestrutura.</p>
    {loading && <p role="status" className="yf-admin__muted">Carregando provedores...</p>}
    {!loading && <div className="yf-admin__source-list">
      {sources.map(source=><div className="yf-admin__source" key={source.id}>
        <div><strong>{source.name}</strong><small>Addon Stremio · Configuração compartilhada</small></div>
        <div className="yf-admin__source-controls">
          <label htmlFor={'egress-route-'+source.id}>
            Rede
            <select id={'egress-route-'+source.id}
              value={source.networkRoute || 'direct'} disabled={pending}
              onChange={event=>void updateRoute(source,event.target.value as StreamAddon['networkRoute'])}>
              <option value="direct">Direto</option>
              {egressBySource[source.id] && <option value="egress">Egress privado</option>}
              {egressBySource[source.id] && <option value="auto">Automático</option>}
            </select>
          </label>
          <label htmlFor={'priority-'+source.id}>Prioridade
            <input id={'priority-'+source.id} aria-label={'Prioridade de '+source.name} type="number" min={1} max={1000}
              value={draftFor(source).priority} disabled={pending}
              onChange={e=>changeDraft(source,'priority',Number(e.target.value))}/>
          </label>
          <label htmlFor={'timeout-'+source.id}>Timeout
            <select id={'timeout-'+source.id} aria-label={'Timeout de '+source.name} disabled={pending}
              value={draftFor(source).timeoutMs}
              onChange={e=>changeDraft(source,'timeoutMs',Number(e.target.value))}>
              {Array.from(new Set([1000,2000,3000,5000,7000,10000,draftFor(source).timeoutMs])).sort((a,b)=>a-b)
                .map(ms=><option key={ms} value={ms}>{ms/1000}s</option>)}
            </select>
          </label>
          <button type="button" disabled={pending || !policyDrafts[source.id]} onClick={()=>void savePolicy(source)}>Salvar regras</button>
          <button disabled={pending} type="button" onClick={()=>remove(source)}>Remover</button>
        </div>
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
      <p className="yf-admin__muted">Use o link completo do addon, inclusive o caminho de configurações antes de /manifest.json. Não exponha esse link a outros usuários. A reprodução permanece limitada a MP4 HTTPS com Range; HLS, MKV e addons que exigem headers especiais ainda não são suportados.</p>
    </div>
    {error && <p role="alert" className="yf-alert">{error}</p>}
    {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
  </section>
}
