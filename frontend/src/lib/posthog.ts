import posthog from 'posthog-js'

let initialized = false

export function initPostHog(key: string) {
  if (initialized || !key) return
  initialized = true
  posthog.init(key, {
    api_host: 'https://us.i.posthog.com',
    autocapture: true,
    capture_pageview: 'history_change',
    capture_pageleave: true,
    person_profiles: 'identified_only',
  })
}

export { posthog }

export function isPostHogEnabled() {
  return initialized
}

export function trackChat(
  question: string,
  result: { model?: string; sources?: string[] },
  latencyMs: number,
  error?: string,
) {
  if (!initialized) return
  posthog.capture('$ai_generation', {
    $ai_model: result.model ?? 'unknown',
    $ai_input: question,
    $ai_output_choices: error ? undefined : 'response received',
    $ai_latency_ms: latencyMs,
    $ai_error: error,
    sources_count: result.sources?.length ?? 0,
  })
}
