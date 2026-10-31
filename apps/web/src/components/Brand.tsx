import { Link } from 'react-router'

export default function Brand({ compact = false }: { compact?: boolean }) {
  return <Link to="/" className={'yf-brand' + (compact ? ' yf-brand--compact' : '')} aria-label="YuFlix, início">
    <span>Yu</span><strong>Flix</strong>
  </Link>
}
