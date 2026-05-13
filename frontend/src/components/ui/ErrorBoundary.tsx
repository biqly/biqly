import { Component, type ErrorInfo, type ReactNode } from 'react'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children

    return (
      <section className="card empty-state" role="alert">
        <h2>Beklenmeyen bir arayüz hatası oluştu</h2>
        <p>{this.state.error.message || 'Bu modül yüklenirken hata aldı.'}</p>
        <button type="button" className="btn btn-sm" onClick={() => this.setState({ error: null })}>
          Tekrar dene
        </button>
      </section>
    )
  }
}
