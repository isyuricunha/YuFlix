import React from 'react'
import { createRoot } from 'react-dom/client'
import '@isyuricunha/yutheme/styles'
import '@fontsource-variable/dm-sans/wght.css'
import '@fontsource/instrument-serif/400-italic.css'
import './style.css'
import App from './App'
import { BrowserRouter } from 'react-router'

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter><App /></BrowserRouter>
  </React.StrictMode>,
)
