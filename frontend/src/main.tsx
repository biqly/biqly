import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import AIJobTracker from './components/AIJobTracker'
import { AIJobsProvider } from './hooks/useAIJobs'
import { I18nProvider } from './i18n'
import { ThemeProvider } from './theme'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider>
      <ThemeProvider>
        <AIJobsProvider>
          <App />
          <AIJobTracker />
        </AIJobsProvider>
      </ThemeProvider>
    </I18nProvider>
  </React.StrictMode>,
)
