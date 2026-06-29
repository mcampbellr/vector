import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import { App } from './App'
import { ThemeProvider } from './context/ThemeContext'

const container = document.getElementById('root')
if (!container) {
  throw new Error('root element not found')
}

createRoot(container).render(
  <StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </StrictMode>,
)
