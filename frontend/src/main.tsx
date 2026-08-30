import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'
import App from './App'
import { initPostHog } from './lib/posthog'

fetch('/api/config')
  .then(r => r.json())
  .then(c => initPostHog(c.posthog_key))
  .catch(() => {})
  .then(() => {
    createRoot(document.getElementById('root')!).render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
  });

