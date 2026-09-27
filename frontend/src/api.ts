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
  last_purchase_date: string
  days_since_last_purchase: number
  estimated_days_remaining: number | null
  median_repurchase_days: number
  mean_repurchase_days: number
  interval_variance: number
  purchase_regularity: number
  average_quantity: number
  median_quantity: number
  prediction_confidence: number
}

export type Suggestion = {
  id: string
  kind: string
  message: string
  section: string
  status: 'pending' | 'accepted' | 'dismissed' | 'fulfilled'
  items: { product_id: string; decision_id: string }[]
  created_at: string
}

export type CartLine = {
  product_id: string
  in_cart: boolean
  in_list: boolean
  cart_quantity: number
  auto_added: boolean
}

export type DecisionResult = {
  action: string
  confidence: number
  probability?: number
  provider: string
  model: string
  latency_ms: number
  cost_usd?: number | null
  metadata?: { probabilities?: Record<string, number>; usage?: { input_tokens?: number; output_tokens?: number } }
}

export type DecisionLog = {
  id: string
  evaluation_id: string
  product_id: string
  engine: string
  shadow: boolean
  decision_context: {
    decision_context_version: string
    product: Prediction & { running_low_snooze_hours_remaining: number | null; last_suggestion_hours_ago: number | null }
  }
  decision_result: DecisionResult | null
  policy_result: { allowed: boolean; action: string; reason: string } | null
  executed_action: string
  error?: string
  latency_ms: number
  created_at: string
}

export type Rate = { numerator: number; denominator: number; rate: number | null }

export type Metrics = {
  suggestion_acceptance: Rate
  suggestion_dismissal_false_positive_proxy: Rate
  shadow_valid_pair_coverage: Rate
  shadow_action_agreement: Rate
  purchase_conversion: Record<string, Rate>
  engines: Record<string, { decisions: number; errors: number; average_latency_ms: number; known_cost_usd: number | null }>
  running_low: {
    questions_shown: number
    confirmed_prediction_precision: Rate
    denied: Rate
    purchase_within_7d_after_confirm: Rate
  }
}

export type Evaluation = { evaluation_id: string; shadow_enabled: boolean; shadow_warning?: string }

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

export const askKind = 'ASK_IF_RUNNING_LOW'
export const seedUser = '00000000-0000-4000-8000-000000000001'

// The backend caps list pages at 100; a demo catalogue fits in one page.
export const fetchProducts = () => request<Product[]>('/products?limit=100')

export const messageOf = (error: unknown) => error instanceof Error ? error.message : 'Something went wrong.'
export const percent = (value: number | null | undefined) => value == null ? '—' : `${Math.round(value * 100)}%`
export const shortId = (value: string) => `${value.slice(0, 8)}…${value.slice(-4)}`
export const dateLabel = (value: string) => new Date(value).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

const categoryIcons: Record<string, string> = {
  milk: '🥛', dairy: '🥛', eggs: '🥚', beverages: '🥤', bakery: '🍞', pantry: '🍚', household: '🧺',
  'personal care': '🧴', fruit: '🍌', electronics: '📺',
}
export const productIcon = (product?: Product) => {
  if (!product) return '🛒'
  if (/water/i.test(product.name)) return '💧'
  if (/coffee/i.test(product.name)) return '☕'
  if (/tissue|toilet/i.test(product.name)) return '🧻'
  return categoryIcons[product.category.toLowerCase()] ?? '🛍️'
}
