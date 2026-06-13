import './index.css'

import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'

import App from './App'
import AIJobTracker from './components/AIJobTracker'
import { AuthProvider } from './components/auth/AuthProvider'
import { AppUpdateGate } from './components/ui/AppUpdateGate'
import { AIJobsProvider } from './hooks/useAIJobs'
import { ConfirmProvider } from './hooks/useConfirm'
import { ShortcutsProvider } from './hooks/useKeyboardShortcuts'
import { ToastProvider } from './hooks/useToast'
import { I18nProvider } from './i18n'
import { ThemeProvider } from './theme'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider>
      <ThemeProvider>
        <ConfirmProvider>
          <ToastProvider>
            <ShortcutsProvider>
              <BrowserRouter>
                <AuthProvider>
                  <AIJobsProvider>
                    <App />
                    <AppUpdateGate />
                    <AIJobTracker />
                  </AIJobsProvider>
                </AuthProvider>
              </BrowserRouter>
            </ShortcutsProvider>
          </ToastProvider>
        </ConfirmProvider>
      </ThemeProvider>
    </I18nProvider>
  </React.StrictMode>,
)
