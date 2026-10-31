import { Navigate, Route, Routes, useLocation } from 'react-router'
import type { ReactNode } from 'react'
import { AuthProvider, useAuth } from './auth/AuthContext'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import AdminPage from './pages/AdminPage'

function LoadingScreen() {
  return <div className="opening-screen" role="status" aria-live="polite">
    <span className="opening-screen__mark" aria-hidden="true">Y</span>
    <span>Preparando sua experiência</span>
  </div>
}

function RequireUser({ children, admin = false }: { children: ReactNode; admin?: boolean }) {
  const { state, user } = useAuth()
  const location = useLocation()

  if (state === 'checking') return <LoadingScreen />
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  if (admin && user.role !== 'admin') return <Navigate to="/" replace />
  return children
}

function GuestOnly({ children }: { children: ReactNode }) {
  const { state, user } = useAuth()
  if (state === 'checking') return <LoadingScreen />
  if (user) return <Navigate to="/" replace />
  return children
}

export default function App() {
  return <AuthProvider>
    <Routes>
      <Route path="/login" element={<GuestOnly><LoginPage /></GuestOnly>} />
      <Route path="/" element={<RequireUser><HomePage /></RequireUser>} />
      <Route path="/admin" element={<RequireUser admin><AdminPage /></RequireUser>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  </AuthProvider>
}
