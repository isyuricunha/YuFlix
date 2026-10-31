import { Link } from 'react-router'

type Props = { compact?: boolean; light?: boolean }
export default function Brand({ compact = false, light = false }: Props) {
  return <Link to="/" className={'yuflix-brand' + (compact ? ' yuflix-brand--compact' : '') + (light ? ' yuflix-brand--light' : '')} aria-label="YuFlix, início">
    <span className="yuflix-brand__symbol" aria-hidden="true"><span>Y</span></span>
    <span className="yuflix-brand__name">Yu<span>Flix</span></span>
  </Link>
}
