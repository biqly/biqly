import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import AIJobTracker from './components/AIJobTracker'
import { AuthProvider } from './components/auth/AuthProvider'
import { AIJobsProvider } from './hooks/useAIJobs'
import { ConfirmProvider } from './hooks/useConfirm'
import { ShortcutsProvider } from './hooks/useKeyboardShortcuts'
import { ToastProvider } from './hooks/useToast'
import { I18nProvider } from './i18n'
import { ThemeProvider } from './theme'
import './index.css'
import './styles/loading.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider>
      <ThemeProvider>
        <ConfirmProvider>
          <ToastProvider>
            <ShortcutsProvider>
              <AIJobsProvider>
                <BrowserRouter>
                  <AuthProvider>
                    <App />
                    <AIJobTracker />
                  </AuthProvider>
                </BrowserRouter>
              </AIJobsProvider>
            </ShortcutsProvider>
          </ToastProvider>
        </ConfirmProvider>
      </ThemeProvider>
    </I18nProvider>
  </React.StrictMode>,
)

