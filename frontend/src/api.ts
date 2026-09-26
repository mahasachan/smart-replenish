export type User = {
  id: string
  timezone: string
  suggestion_enabled: boolean
  notification_enabled: boolean
}

export type Product = {
  id: string
  sku: string
  name: string
  category: string
  brand: string
  unit: string
  replenishable_score: number
  available: boolean
}

export type Prediction = {
  product_id: string
  eligible: boolean
  reason?: string
  total_purchase_count: number
  estimated_days_remaining: number | null
  median_repurchase_days: number
  prediction_confidence: number
}

export type Suggestion = {
  id: string
  kind: string
  message: string
  section: string
  status: string
  items: { product_id: string; decision_id: string }[]
  created_at: string
}

export type DecisionLog = {
  id: string
  evaluation_id: string
  product_id: string
  engine: string
  shadow: boolean
  decision_result: {
    action: string
    confidence: number
    probability?: number
    provider: string
    model: string
    latency_ms: number
    cost_usd?: number | null
  } | null
  policy_result: { allowed_action: string; reason: string } | null
  executed_action: string
  error?: string
  latency_ms: number
  created_at: string
}

export type Metrics = {
  suggestion_acceptance: { numerator: number; denominator: number; rate: number | null }
  suggestion_dismissal_false_positive_proxy: { numerator: number; denominator: number; rate: number | null }
  shadow_valid_pair_coverage: { numerator: number; denominator: number; rate: number | null }
  shadow_action_agreement: { numerator: number; denominator: number; rate: number | null }
  engines: Record<string, { decisions: number; errors: number; average_latency_ms: number; known_cost_usd: number | null }>
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    headers: { ...(init?.body ? { 'Content-Type': 'application/json' } : {}), ...init?.headers },
  })
  const text = await response.text()
  let body: unknown
  try { body = text ? JSON.parse(text) : undefined } catch { body = text }
  if (!response.ok) {
    const message = typeof body === 'object' && body !== null && 'error' in body
      ? String((body as { error: unknown }).error)
      : `Request failed (${response.status})`
    throw new Error(message)
  }
  return body as T
}

export const json = (method: string, body: unknown, headers?: Record<string, string>): RequestInit => ({
  method,
  body: JSON.stringify(body),
  headers,
})

export const seedUser = '00000000-0000-4000-8000-000000000001'
export const seedProducts: Product[] = [
  { id: '00000000-0000-4000-8000-000000000101', sku: 'milk-001', name: 'Milk', category: 'milk', brand: '', unit: 'carton', replenishable_score: 0.98, available: true },
  { id: '00000000-0000-4000-8000-000000000102', sku: 'eggs-001', name: 'Eggs', category: 'eggs', brand: '', unit: 'box', replenishable_score: 0.95, available: true },
  { id: '00000000-0000-4000-8000-000000000103', sku: 'tv-001', name: 'Television', category: 'electronics', brand: '', unit: 'piece', replenishable_score: 0.01, available: true },
]
