import { useEffect, useState } from 'react'
import { getServerStatus } from './api'

type Connection =
  | { state: 'loading' }
  | { state: 'connected'; version: string }
  | { state: 'error'; message: string }

export default function App() {
  const [connection, setConnection] = useState<Connection>({ state: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    getServerStatus(controller.signal)
      .then(({ version }) => {
        if (!controller.signal.aborted) setConnection({ state: 'connected', version })
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setConnection({
            state: 'error',
            message: error instanceof Error ? error.message : 'Could not reach the server.',
          })
        }
      })
    return () => controller.abort()
  }, [attempt])

  function refresh() {
    setConnection({ state: 'loading' })
    setAttempt((value) => value + 1)
  }

  return (
    <div className="shell">
      <header className="header">
        <a className="brand" href="/">FaultHarbor<span className="brand-dot" aria-hidden="true">.</span></a>
        <span className="badge">Open source</span>
      </header>
      <main>
        <p className="eyebrow">Infrastructure diagnostics</p>
        <h1>Infrastructure overview.</h1>
        <p className="intro">Investigate failures across services, networks, and routes.</p>

        <section className="panel" aria-labelledby="status-title">
          <div className="panel-heading">
            <h2 id="status-title">Server connection</h2>
            <button onClick={refresh} disabled={connection.state === 'loading'}>
              {connection.state === 'loading' ? 'Checking…' : 'Check again'}
            </button>
          </div>
          <div role="status" aria-live="polite" className="connection">
            <span className={`indicator ${connection.state}`} aria-hidden="true" />
            {connection.state === 'loading' && <p>Connecting to FaultHarbor…</p>}
            {connection.state === 'connected' && <p>Connected <span className="version">{connection.version}</span></p>}
            {connection.state === 'error' && <div><p>Connection unavailable</p><p className="detail">{connection.message} Check the server connection and try again.</p></div>}
          </div>
        </section>

        <section className="next" aria-labelledby="next-title">
          <div>
            <h2 id="next-title">No infrastructure sources connected</h2>
            <p>Service, network, and route findings will appear here when source data is available.</p>
          </div>
        </section>
      </main>
      <footer>FaultHarbor · Open-source infrastructure diagnostics.</footer>
    </div>
  )
}
