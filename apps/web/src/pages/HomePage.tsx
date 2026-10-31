import { Link } from 'react-router'
import { useState } from 'react'
import CatalogHome from '../components/CatalogHome'
import { useAuth } from '../auth/AuthContext'

export default function HomePage() {
  const { user } = useAuth()
  const [hasCatalog, setHasCatalog] = useState(false)

  return <div className="yf-home">
    {!hasCatalog && <section className="yf-feature" aria-labelledby="feature-title">
      <div className="yf-feature__copy">
        <span className="yf-feature__eyebrow">YUFLIX · I WATCH WHAT I WANT.</span>
        <h1 id="feature-title">Seu streaming.<br /><span>Seu jeito.</span></h1>
        <p>Uma biblioteca para descobrir o que você quer assistir. Conecte seus catálogos e escolha suas fontes.</p>
        {user?.role === 'admin' ? <Link className="yf-feature__action" to="/admin?section=catalogs">Configurar catálogos <span aria-hidden="true">→</span></Link> :
          <span className="yf-feature__status">O catálogo ainda está sendo preparado.</span>}
      </div>
    </section>}
    <CatalogHome isAdmin={user?.role === 'admin'} onCatalogPresent={setHasCatalog} />
  </div>
}
