import { useEffect, useState } from 'react'
import './App.css'

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

function App() {
  const [apiStatus, setApiStatus] = useState<string>('checking…')

  useEffect(() => {
    fetch(`${API_URL}/health`)
      .then((r) => r.json())
      .then((d) => setApiStatus(d.status === 'ok' ? '✅ API reachable' : '⚠️ unexpected response'))
      .catch(() => setApiStatus('❌ API unreachable'))
  }, [])

  return (
    <div style={{ fontFamily: 'sans-serif', padding: '2rem' }}>
      <h1>Seven Spade 🂡</h1>
      <p>API: {apiStatus}</p>
    </div>
  )
}

export default App
