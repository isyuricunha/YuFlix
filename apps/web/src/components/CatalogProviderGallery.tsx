import { useEffect, useState } from 'react'
import { api, type CatalogProvider, type CatalogSource } from '../api'

type Props = {
  onInstalled: (source: CatalogSource) => void
}

export default function CatalogProviderGallery({ onInstalled }: Props) {
  const [providers, setProviders] = useState<CatalogProvider[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [filter, setFilter] = useState('Todos')

  useEffect(() => {
    let mounted = true
    api<{providers: CatalogProvider[]}>('/admin/catalog/providers')
      .then(data => { if (mounted) setProviders(data.providers) })
      .catch(() => { if (mounted) setError('Não foi possível consultar as integrações disponíveis.') })
      .finally(() => { if (mounted) setLoading(false) })
    return () => { mounted = false }
  }, [])

  async function install(item: CatalogProvider) {
    if (pending || item.installed || !item.ready) return
    setPending(item.id); setError(''); setNotice('')
    try {
      const data = await api<{source: CatalogSource}>('/admin/catalog/providers/'+item.id, {method:'POST'})
      onInstalled(data.source)
      setProviders(items => items.map(p => p.id === item.id ? {...p, installed:true} : p))
      setNotice(item.name + ' adicionado ao YuFlix.')
    } catch {
      setError('Não foi possível conectar ' + item.name + '. O serviço pode estar indisponível no momento.')
    } finally { setPending('') }
  }

  const categories = ['Todos', ...new Set(providers.map(item => item.category))]
  const results = providers.filter(item => filter === 'Todos' || item.category === filter)

  return <section className="yf-provider-market" aria-labelledby="provider-market-title">
    <div className="yf-provider-market__heading">
      <div>
        <span className="yf-kicker">BIBLIOTECA DE INTEGRAÇÕES</span>
        <h2 id="provider-market-title">Adicionar conteúdo</h2>
        <p>Escolha um provedor. O YuFlix cuida da conexão e das atualizações, sem precisar informar URLs.</p>
      </div>
      <span className="yf-provider-market__total">{providers.filter(p => p.installed).length} conectados</span>
    </div>
    {providers.length > 0 && <div className="yf-provider-market__filters" role="group" aria-label="Filtrar integrações">
      {categories.map(category => <button key={category} type="button" aria-pressed={filter===category}
        onClick={() => setFilter(category)}>{category}</button>)}
    </div>}
    {loading && <p role="status" className="yf-admin__muted">Consultando integrações disponíveis…</p>}
    {!loading && <div className="yf-provider-market__grid">
      {results.map(item => <article className="yf-provider-market__item" key={item.id}>
        <div className="yf-provider-market__monogram" aria-hidden="true">{item.name.slice(0,1)}</div>
        <div className="yf-provider-market__info">
          <span className="yf-provider-market__category">{item.category}</span>
          <h3>{item.name}</h3>
          <p>{item.description}</p>
        </div>
        {item.note && <p className="yf-provider-market__note">{item.note}</p>}
        <button type="button" disabled={!item.ready || item.installed || !!pending} onClick={()=>void install(item)}>
          {pending===item.id ? 'Conectando…' : item.installed ? 'Conectado' : item.ready ? 'Adicionar' : 'Em preparação'}
        </button>
      </article>)}
    </div>}
    {notice && <p role="status" className="yf-admin__notice">{notice}</p>}
    {error && <p role="alert" className="yf-alert">{error}</p>}
  </section>
}
