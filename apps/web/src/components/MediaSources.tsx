import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api, type MediaCandidate, type PlaybackSession, type ProviderStatus } from '../api'

export default function MediaSources({type,videoId,isAdmin}:{
  type:'movie'|'series';videoId:string;isAdmin:boolean
}) {
  const [candidates,setCandidates]=useState<MediaCandidate[]>([])
  const [providers,setProviders]=useState(0)
  const [statuses,setStatuses]=useState<ProviderStatus[]>([])
  const [loading,setLoading]=useState(false)
  const [checking,setChecking]=useState<string|null>(null)
  const [session,setSession]=useState<PlaybackSession|null>(null)
  const [message,setMessage]=useState('')
  useEffect(()=>{
    setCandidates([])
    setProviders(0)
    setStatuses([])
    setSession(null)
    setMessage('')
  },[type,videoId])
  async function discover(){
    setLoading(true);setMessage('');setSession(null);setStatuses([]);setCandidates([])
    try{
      const params='?type='+encodeURIComponent(type)+'&id='+encodeURIComponent(videoId)
      const data=await api<{streams:MediaCandidate[],providersChecked:number,providersFailed:number,providerStatuses?:ProviderStatus[]}>('/media/streams'+params)
      setCandidates(data.streams)
      setProviders(data.providersChecked)
      setStatuses(data.providerStatuses??[])
      if(!data.streams.length){
        setMessage(data.providersChecked===0 ? 'Nenhum provedor de vídeo cadastrado.' :
          'Nenhuma fonte reproduzível foi encontrada. Confira o resultado de cada provedor abaixo.')
      }
    } catch {
      setMessage('Não foi possível consultar as fontes de vídeo.')
    } finally {setLoading(false)}
  }
  async function play(stream:MediaCandidate){
    if(checking)return
    setChecking(stream.sourceId+':'+stream.index);setMessage('');setSession(null)
    try{
      const data=await api<PlaybackSession>('/media/sessions',{
        method:'POST',body:JSON.stringify({sourceId:stream.sourceId,type,id:videoId,index:stream.index,candidateKey:stream.key}),
      })
      if(!data.path.startsWith('/api/v1/media/play/'))throw new Error('invalid media route')
      setSession(data)
    }catch{
      setMessage('A fonte não passou na verificação de MP4 com busca por posição. Tente outra.')
    }finally{setChecking(null)}
  }
  return <section className="yf-playback" aria-labelledby="playback-title">
    <div className="yf-playback__heading">
      <div><span className="yf-kicker">REPRODUÇÃO</span><h2 id="playback-title">Fontes de vídeo</h2></div>
      <button type="button" disabled={loading || !!checking} onClick={()=>void discover()}>
        {loading?'Procurando...':candidates.length?'Atualizar fontes':'Buscar fontes'}
      </button>
    </div>
    <p className="yf-playback__description">Direct Play experimental. MP4 HTTPS, inclusive URLs sem extensão verificadas por Range, é aceito, sem baixar o arquivo inteiro ou usar transcode.</p>
    {message && <p className="yf-playback__notice" role="status">{message}
      {providers===0&&isAdmin&&<span> <Link to="/admin">Configure um provedor em Administração → Reprodução.</Link></span>}
    </p>}
    {!!statuses.length && <div className="yf-playback__provider-results" aria-label="Resultado dos provedores">
      {statuses.filter(source=>source.state!=='ok').map((source,index)=><p className="yf-playback__notice" key={source.name+index} role="status">
        <strong>{source.name}:</strong>{' '}
        {source.state==='empty' ? 'Nenhum vídeo retornado.' :
          source.state==='unsupported' ? source.received+' fonte(s) retornada(s), mas nenhuma compatível com o Direct Play MP4 atual.' :
          source.reason==='access_denied' ? 'Acesso recusado pelo provedor (HTTP '+(source.httpStatus||403)+').' :
          source.reason==='not_found' ? 'Conteúdo não encontrado pelo addon (HTTP 404).' :
          source.reason==='rate_limited' ? 'Provedor limitou temporariamente as consultas (HTTP 429).' :
          source.reason==='timeout' ? 'Tempo limite da consulta atingido.' :
          source.reason==='unsafe_address' ? 'O destino foi bloqueado pela política de segurança.' :
          source.reason==='invalid_response' ? 'Resposta inválida ou incompatível com o protocolo Stremio.' :
          source.reason==='upstream_unavailable' ? 'Servidor do addon indisponível (HTTP '+(source.httpStatus||500)+').' :
          source.httpStatus ? 'Erro HTTP '+source.httpStatus+' retornado pelo addon.' :
          'Não foi possível conectar ao addon.'}
      </p>)}
    </div>}
    {!!candidates.length && <div className="yf-playback__sources">
      {candidates.map(item=><div className="yf-playback__source" key={item.sourceId+':'+item.index}>
        <div><strong>{item.name}</strong><span>{item.sourceName}{item.title?' · '+item.title:''}</span></div>
        <button type="button" disabled={!!checking||loading} onClick={()=>void play(item)}>
          {checking===item.sourceId+':'+item.index?'Validando...':'Testar Direct Play'}
        </button>
      </div>)}
    </div>}
    {session&&<div className="yf-playback__active">
      <div className="yf-playback__active-heading"><strong>{session.name}</strong><span>MP4 · Direct Play</span></div>
      <video controls playsInline preload="metadata" src={session.path}
        onError={()=>setMessage('O navegador não conseguiu reproduzir esta fonte ou os codecs não são compatíveis.')}
        aria-label="Reprodutor YuFlix" />
      <p>A verificação inicial passou. Confirme início, áudio, busca por posição e final no seu dispositivo.</p>
    </div>}
  </section>
}
