import { useState } from 'react'
import QueryBuilder from './components/QueryBuilder'
import Dashboard from './components/Dashboard'
import SavedQuestions from './components/SavedQuestions'
import AIQuery from './components/AIQuery'
import Datasources from './components/Datasources'
import Metadata from './components/Metadata'

type Tab = 'datasources' | 'metadata' | 'query' | 'ai' | 'dashboard' | 'saved'

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('datasources')

  const tabs: { key: Tab; label: string }[] = [
    { key: 'datasources', label: 'Datasources' },
    { key: 'metadata', label: 'Metadata' },
    { key: 'query', label: 'Query Builder' },
    { key: 'ai', label: 'AI Query' },
    { key: 'dashboard', label: 'Dashboard' },
    { key: 'saved', label: 'Saved' },
  ]

  return (
    <div className="app">
      <header className="header">
        <h1>📊 Biqly</h1>
        <nav className="nav">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              className={activeTab === tab.key ? 'active' : ''}
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </header>
      <main className="main">
        {activeTab === 'datasources' && <Datasources />}
        {activeTab === 'metadata' && <Metadata />}
        {activeTab === 'query' && <QueryBuilder />}
        {activeTab === 'ai' && <AIQuery />}
        {activeTab === 'dashboard' && <Dashboard />}
        {activeTab === 'saved' && <SavedQuestions />}
      </main>
    </div>
  )
}

export default App
