import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import AIJobTracker from './components/AIJobTracker'
import { AuthProvider } from './components/auth/AuthProvider'
import { AIJobsProvider } from './hooks/useAIJobs'
import { ConfirmProvider } from './hooks/useConfirm'
import { I18nProvider } from './i18n'
import { ThemeProvider } from './theme'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider>
      <ThemeProvider>
        <ConfirmProvider>
          <AIJobsProvider>
            <AuthProvider>
              <App />
              <AIJobTracker />
            </AuthProvider>
          </AIJobsProvider>
        </ConfirmProvider>
      </ThemeProvider>
    </I18nProvider>
  </React.StrictMode>,
)

