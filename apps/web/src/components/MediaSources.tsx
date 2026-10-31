import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { api, type MediaCandidate, type PlaybackSession } from '../api'

export default function MediaSources({type,videoId,isAdmin}:{
  type:'movie'|'series';videoId:string;isAdmin:boolean
}) {
  const [candidates,setCandidates]=useState<MediaCandidate[]>([])
  const [providers,setProviders]=useState(0)
  const [failures,setFailures]=useState(0)
  const [loading,setLoading]=useState(false)
  const [checking,setChecking]=useState<string|null>(null)
  const [session,setSession]=useState<PlaybackSession|null>(null)
  const [message,setMessage]=useState('')
  useEffect(()=>{
    setCandidates([])
    setProviders(0)
    setFailures(0)
    setSession(null)
    setMessage('')
  },[type,videoId])
  async function discover(){
    setLoading(true);setMessage('');setSession(null)
    try{
      const params='?type='+encodeURIComponent(type)+'&id='+encodeURIComponent(videoId)
      const data=await api<{streams:MediaCandidate[],providersChecked:number,providersFailed:number}>('/media/streams'+params)
      setCandidates(data.streams)
      setProviders(data.providersChecked)
      setFailures(data.providersFailed)
      if(!data.streams.length){
        setMessage(data.providersChecked===0 ? 'Nenhum provedor de vídeo cadastrado.' :
          'Os provedores não retornaram fontes MP4 compatíveis para este título.')
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
    <p className="yf-playback__description">Direct Play experimental. Apenas MP4 HTTPS com Range é aceito, sem baixar o arquivo inteiro ou usar transcode.</p>
    {message && <p className="yf-playback__notice" role="status">{message}
      {providers===0&&isAdmin&&<span> <Link to="/admin">Configure um provedor em Administração → Reprodução.</Link></span>}
    </p>}
    {!!failures && <p className="yf-playback__notice">{failures} provedor(es) não responderam.</p>}
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
