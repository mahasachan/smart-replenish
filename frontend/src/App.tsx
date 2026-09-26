import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { json, request, seedProducts, seedUser, type DecisionLog, type Metrics, type Prediction, type Product, type Suggestion, type User } from './api'

const userStoreKey = 'smartreplenish.simulator.users'
const productStoreKey = 'smartreplenish.simulator.products'
const readIds = (key: string, fallback: string[]) => {
  try { return [...new Set([...fallback, ...JSON.parse(localStorage.getItem(key) ?? '[]') as string[]])] }
  catch { return fallback }
}
const saveId = (key: string, id: string) => {
  const ids = readIds(key, [])
  localStorage.setItem(key, JSON.stringify([id, ...ids.filter((value) => value !== id)]))
}
const readProducts = (): Product[] => {
  try { return [...seedProducts, ...JSON.parse(localStorage.getItem(productStoreKey) ?? '[]') as Product[]] }
  catch { return seedProducts }
}
const messageOf = (error: unknown) => error instanceof Error ? error.message : 'Something went wrong.'
const percent = (value: number | null | undefined) => value == null ? '—' : `${Math.round(value * 100)}%`
const shortId = (value: string) => `${value.slice(0, 8)}…${value.slice(-4)}`
const dateLabel = (value: string) => new Date(value).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

function App() {
  const queryClient = useQueryClient()
  const [userIds, setUserIds] = useState(() => readIds(userStoreKey, [seedUser]))
  const [products, setProducts] = useState<Product[]>(readProducts)
  const [userId, setUserId] = useState(seedUser)
  const [productId, setProductId] = useState(seedProducts[0].id)
  const [notice, setNotice] = useState('')
  const [evaluation, setEvaluation] = useState<{ evaluation_id: string; shadow_enabled: boolean; shadow_warning?: string } | null>(null)

  const health = useQuery({ queryKey: ['health'], queryFn: () => request<{ status: string }>('/healthz'), refetchInterval: 10_000 })
  const predictions = useQuery({ queryKey: ['predictions', userId], queryFn: () => request<Prediction[]>(`/users/${userId}/replenishment`), enabled: Boolean(userId) })
  const suggestions = useQuery({ queryKey: ['suggestions', userId], queryFn: () => request<Suggestion[]>(`/users/${userId}/suggestions?limit=50`), enabled: Boolean(userId) })
  const history = useQuery({ queryKey: ['history', userId], queryFn: () => request<DecisionLog[]>(`/users/${userId}/decision-history?limit=100`), enabled: Boolean(userId) })
  const metrics = useQuery({ queryKey: ['metrics', userId], queryFn: () => request<Metrics>(`/users/${userId}/evaluation-metrics`), enabled: Boolean(userId) })

  const invalidateUserData = async () => {
    await Promise.all(['predictions', 'suggestions', 'history', 'metrics'].map((key) => queryClient.invalidateQueries({ queryKey: [key, userId] })))
  }
  const mutation = useMutation({
    mutationFn: ({ path, method, body, headers }: { path: string; method: string; body?: unknown; headers?: Record<string, string> }) =>
      request<unknown>(path, body === undefined ? { method } : json(method, body, headers)),
    onSuccess: async () => { setEvaluation(null); setNotice('Saved to the backend.'); await invalidateUserData() },
    onError: (error) => setNotice(messageOf(error)),
  })
  const evaluate = useMutation({
    mutationFn: () => request<{ evaluation_id: string; shadow_enabled: boolean; shadow_warning?: string }>(`/users/${userId}/decisions/evaluate`, json('POST', {})),
    onSuccess: async (result) => {
      setEvaluation(result)
      setNotice(`Evaluation ${shortId(result.evaluation_id)} completed.`)
      await invalidateUserData()
    },
    onError: (error) => setNotice(messageOf(error)),
  })
  const createUser = useMutation({
    mutationFn: (payload: Omit<User, 'id'>) => request<User>('/users', json('POST', payload)),
    onSuccess: (user) => {
      saveId(userStoreKey, user.id)
      setUserIds(readIds(userStoreKey, [seedUser]))
      setUserId(user.id)
      setEvaluation(null)
      setNotice(`User ${shortId(user.id)} created.`)
    },
    onError: (error) => setNotice(messageOf(error)),
  })
  const createProduct = useMutation({
    mutationFn: (payload: Omit<Product, 'id'>) => request<Product>('/products', json('POST', payload)),
    onSuccess: (product) => {
      const next = [product, ...readProducts().filter((item) => item.id !== product.id)]
      localStorage.setItem(productStoreKey, JSON.stringify(next.filter((item) => !seedProducts.some((seed) => seed.id === item.id))))
      setProducts(next)
      setProductId(product.id)
      setEvaluation(null)
      setNotice(`Product ${product.name} created.`)
    },
    onError: (error) => setNotice(messageOf(error)),
  })

  const productName = (id: string) => products.find((product) => product.id === id)?.name ?? shortId(id)
  const grouped = useMemo(() => {
    const pairs = new Map<string, { productId: string; createdAt: string; rules?: DecisionLog; jev?: DecisionLog }>()
    for (const row of history.data ?? []) {
      const key = `${row.evaluation_id}:${row.product_id}`
      const pair = pairs.get(key) ?? { productId: row.product_id, createdAt: row.created_at }
      if (row.shadow || row.engine.toLowerCase().includes('jev')) pair.jev = row
      else pair.rules = row
      pairs.set(key, pair)
    }
    return [...pairs.values()].slice(0, 12)
  }, [history.data])

  const submit = (event: FormEvent<HTMLFormElement>, action: () => void) => { event.preventDefault(); action() }
  const chooseProduct = productId || seedProducts[0].id
  const working = mutation.isPending || evaluate.isPending || createUser.isPending || createProduct.isPending

  return (
    <main className="app-shell">
      <aside className="rail">
        <div className="brand-mark">S<span>R</span></div>
        <div className="rail-divider" />
        <div className="rail-icon active" title="Decision lab">⌘</div>
        <div className="rail-icon" title="Data setup">＋</div>
        <div className="rail-bottom"><span className="rail-dot" /></div>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div className="crumb"><span>SMART REPLENISH</span><b>/</b><strong>Decision lab</strong></div>
          <div className="topbar-right"><span className={`connection ${health.isError ? 'offline' : ''}`}><i />{health.isLoading ? 'Connecting' : health.isError ? 'API offline' : 'API connected'}</span><span className="env-tag">LOCAL ENVIRONMENT</span></div>
        </header>

        <div className="page-content">
          <section className="hero">
            <div>
              <div className="eyebrow"><span className="eyebrow-line" /> REPLENISHMENT WORKBENCH</div>
              <h1>Decision lab<span>.</span></h1>
              <p>Shape a customer’s purchase history. See what the system predicts, then compare the rule engine with Jev.</p>
            </div>
            <div className="hero-meta"><span className="pulse" /> LIVE WORKSPACE <span className="meta-sep">·</span> LOCAL DATA</div>
          </section>

          <section className="control-strip panel">
            <div className="control-label"><span className="step-number">01</span><div><b>Choose a customer</b><small>Scenario owner</small></div></div>
            <div className="control-fields">
              <label className="field compact"><span>USER ID</span><select value={userId} onChange={(event) => { setUserId(event.target.value); setEvaluation(null) }}>{userIds.map((id) => <option key={id} value={id}>{id === seedUser ? 'Seed customer · Bangkok' : `Customer · ${shortId(id)}`}</option>)}</select></label>
              <span className="field-separator">↗</span>
              <label className="field compact"><span>FOCUS PRODUCT</span><select value={chooseProduct} onChange={(event) => setProductId(event.target.value)}>{products.map((product) => <option key={product.id} value={product.id}>{product.name} · {product.sku}</option>)}</select></label>
            </div>
            <button className="button primary evaluate-button" disabled={working || health.isError} onClick={() => evaluate.mutate()}><span className="button-icon">◈</span>{evaluate.isPending ? 'Evaluating…' : 'Run evaluation'}<span className="button-arrow">↗</span></button>
          </section>

          {notice && <div className={`notice ${notice.includes('failed') || notice.includes('invalid') || notice.includes('not found') || notice.includes('offline') ? 'notice-error' : ''}`}><span>{notice.includes('completed') || notice.includes('created') || notice.includes('Saved') ? '✓' : '!'}</span>{notice}<button aria-label="Dismiss" onClick={() => setNotice('')}>×</button></div>}

          <div className="section-heading"><div><span className="section-kicker">BUILD A SCENARIO</span><h2>Customer data</h2></div><span className="section-note">Inputs are written directly to your local database</span></div>
          <section className="data-grid">
            <article className="panel data-card">
              <div className="card-heading"><div className="card-icon coral">♙</div><div><h3>Customer</h3><p>Create a new profile for a scenario</p></div><span className="card-index">A</span></div>
              <form onSubmit={(event) => submit(event, () => {
                const data = new FormData(event.currentTarget)
                createUser.mutate({ timezone: String(data.get('timezone') || 'Asia/Bangkok'), notification_enabled: false, suggestion_enabled: data.get('suggestions') === 'on' })
              })}>
                <label className="form-field"><span>TIMEZONE</span><input name="timezone" defaultValue="Asia/Bangkok" placeholder="Asia/Bangkok" /></label>
                <label className="check-row"><input type="checkbox" name="suggestions" defaultChecked /><span className="check-box" />Suggestions enabled</label>
                <button className="button secondary" disabled={working}>Create customer <b>＋</b></button>
              </form>
            </article>

            <article className="panel data-card">
              <div className="card-heading"><div className="card-icon blue">▤</div><div><h3>Product</h3><p>Add an item to purchase history</p></div><span className="card-index">B</span></div>
              <form onSubmit={(event) => submit(event, () => {
                const data = new FormData(event.currentTarget)
                createProduct.mutate({ sku: String(data.get('sku')), name: String(data.get('name')), category: String(data.get('category') || 'household'), brand: String(data.get('brand') || ''), unit: String(data.get('unit') || 'piece'), replenishable_score: Number(data.get('score') || 0.8), available: true })
              })}>
                <div className="form-row"><label className="form-field"><span>PRODUCT NAME</span><input name="name" required placeholder="Oat milk" /></label><label className="form-field narrow"><span>SKU</span><input name="sku" required placeholder="OAT-01" /></label></div>
                <div className="form-row"><label className="form-field"><span>CATEGORY</span><input name="category" placeholder="Dairy alternatives" /></label><label className="form-field score-field"><span>REPLENISH SCORE</span><input name="score" type="number" min="0" max="1" step="0.01" defaultValue="0.85" /></label></div>
                <button className="button secondary" disabled={working}>Create product <b>＋</b></button>
              </form>
            </article>

            <article className="panel data-card purchase-card">
              <div className="card-heading"><div className="card-icon gold">↗</div><div><h3>Purchase event</h3><p>Add one dated purchase to the customer’s history</p></div><span className="card-index">C</span></div>
              <form onSubmit={(event) => submit(event, () => {
                const data = new FormData(event.currentTarget)
                const quantity = Number(data.get('quantity'))
                const unitPrice = Number(data.get('unit_price'))
                const purchasedAt = new Date(`${String(data.get('date'))}T12:00:00`).toISOString()
                mutation.mutate({ path: '/purchases', method: 'POST', body: { user_id: userId, purchased_at: purchasedAt, source: 'app', total_amount: quantity * unitPrice, items: [{ product_id: chooseProduct, quantity, unit_price: unitPrice }] }, headers: { 'Idempotency-Key': crypto.randomUUID() } })
              })}>
                <div className="form-row"><label className="form-field"><span>PURCHASE DATE</span><input name="date" type="date" required defaultValue={new Date().toISOString().slice(0, 10)} /></label><label className="form-field narrow"><span>QUANTITY</span><input name="quantity" type="number" min="1" step="1" defaultValue="1" required /></label><label className="form-field narrow"><span>UNIT PRICE · MINOR</span><input name="unit_price" type="number" min="0" step="1" defaultValue="100" required /></label></div>
                <button className="button secondary" disabled={working}>Record purchase <b>＋</b></button>
              </form>
            </article>
          </section>

          <div className="section-heading state-heading"><div><span className="section-kicker">TUNE CURRENT CONDITIONS</span><h2>Scenario controls</h2></div><span className="section-note">Changes affect the next evaluation</span></div>
          <section className="panel state-panel">
            <div className="state-control"><div className="state-icon">⚙</div><div className="state-copy"><b>Suggestion preference</b><small>Allow this customer to receive suggestions</small></div><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { suggestion_enabled: false } })}>Disable</button><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { suggestion_enabled: true } })}>Enable</button></div>
            <div className="state-divider" />
            <div className="state-control"><div className="state-icon">◌</div><div className="state-copy"><b>Notification preference</b><small>Preference only; this system sends no notifications</small></div><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { notification_enabled: false } })}>Opt out</button><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { notification_enabled: true } })}>Opt in</button></div>
            <div className="state-divider" />
            <div className="state-control"><div className="state-icon">◉</div><div className="state-copy"><b>Product availability</b><small>{productName(chooseProduct)} <span className="subtle-id">{shortId(chooseProduct)}</span></small></div><button className="text-action" onClick={() => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { available: false } })}>Mark unavailable</button><button className="text-action" onClick={() => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { available: true } })}>Available</button></div>
            <div className="state-divider" />
            <div className="state-control"><div className="state-icon">⌁</div><div className="state-copy"><b>Replenishability score</b><small>Change eligibility signal for {productName(chooseProduct)}</small></div><button className="text-action" onClick={() => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { replenishable_score: 0.1 } })}>Low · 0.10</button><button className="text-action" onClick={() => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { replenishable_score: 0.95 } })}>High · 0.95</button></div>
            <div className="state-divider" />
            <div className="state-control"><div className="state-icon">☷</div><div className="state-copy"><b>Shopping state</b><small>Set cart and list membership</small></div><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: true, in_list: false } })}>In cart</button><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: false, in_list: true } })}>On list</button><button className="text-action" onClick={() => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: false, in_list: false } })}>Clear</button></div>
          </section>

          <div className="section-heading results-heading"><div><span className="section-kicker">SYSTEM OUTPUT</span><h2>Evaluation results</h2></div><div className="output-status">{evaluation ? <><i className="status-dot" /> LAST RUN {shortId(evaluation.evaluation_id)}</> : 'RUN AN EVALUATION TO REFRESH'}</div></div>

          {evaluation?.shadow_warning && <div className="warning-banner"><span>!</span><div><b>Jev shadow comparison unavailable</b><small>{evaluation.shadow_warning}</small></div></div>}
          {evaluation && !evaluation.shadow_enabled && !evaluation.shadow_warning && <div className="warning-banner muted-warning"><span>i</span><div><b>Jev is disabled</b><small>Add OPENROUTER_API_KEY to backend/.env and restart the backend to compare shadow decisions.</small></div></div>}

          <section className="results-grid">
            <article className="panel prediction-panel">
              <div className="result-header"><div><span className="result-overline">01 / FORECAST</span><h3>Replenishment signals</h3></div><span className="live-tag">{predictions.data?.length ?? 0} ITEMS</span></div>
              {predictions.isLoading ? <div className="empty-state">Loading purchase patterns…</div> : predictions.isError ? <div className="empty-state error-text">{messageOf(predictions.error)}</div> : !predictions.data?.length ? <div className="empty-state">No purchase history yet. Add dated purchase events to build a forecast.</div> : <div className="prediction-list">{predictions.data.map((item) => <div className="prediction-row" key={item.product_id}><div className="prediction-product"><span className="product-bullet" /><div><b>{productName(item.product_id)}</b><small>{item.total_purchase_count} purchase occasions · {item.eligible ? 'eligible' : item.reason?.replaceAll('_', ' ')}</small></div></div><div className="prediction-stats"><div><small>DAYS LEFT</small><b>{item.estimated_days_remaining == null ? '—' : item.estimated_days_remaining.toFixed(1)}</b></div><div><small>CONFIDENCE</small><b>{percent(item.prediction_confidence)}</b></div></div></div>)}</div>}
            </article>

            <article className="panel metrics-panel">
              <div className="result-header"><div><span className="result-overline">02 / QUALITY</span><h3>Observed outcomes</h3></div><span className="metrics-icon">⌁</span></div>
              {metrics.isLoading ? <div className="empty-state">Loading metrics…</div> : metrics.isError ? <div className="empty-state error-text">{messageOf(metrics.error)}</div> : <div className="metric-list">
                <Metric label="Shadow agreement" value={percent(metrics.data?.shadow_action_agreement.rate)} detail={`${metrics.data?.shadow_action_agreement.numerator ?? 0} / ${metrics.data?.shadow_action_agreement.denominator ?? 0} paired`} />
                <Metric label="Pair coverage" value={percent(metrics.data?.shadow_valid_pair_coverage.rate)} detail={`${metrics.data?.shadow_valid_pair_coverage.numerator ?? 0} valid comparisons`} />
                <Metric label="Suggestion acceptance" value={percent(metrics.data?.suggestion_acceptance.rate)} detail={`${metrics.data?.suggestion_acceptance.numerator ?? 0} accepted`} />
                <Metric label="Suggestion dismissal" value={percent(metrics.data?.suggestion_dismissal_false_positive_proxy.rate)} detail="False-positive proxy" />
              </div>}
              <p className="metric-footnote">Descriptive rates only. Agreement compares selected actions; it does not measure correctness.</p>
            </article>
          </section>

          <section className="panel comparison-panel">
            <div className="result-header comparison-head"><div><span className="result-overline">03 / DECISION TRACE</span><h3>Rules vs Jev</h3><p>Matched by evaluation and product so every comparison uses the same context.</p></div><div className="engine-legend"><span><i className="legend-rule" /> PRODUCTION RULES</span><span><i className="legend-jev" /> JEV SHADOW</span></div></div>
            {history.isLoading ? <div className="empty-state">Loading decision history…</div> : history.isError ? <div className="empty-state error-text">{messageOf(history.error)}</div> : grouped.length === 0 ? <div className="empty-state">No decisions yet. Add enough history and run an evaluation to see the comparison.</div> : <div className="comparison-table"><div className="comparison-columns"><span>PRODUCT / EVALUATION</span><span>RULE ENGINE</span><span>JEV · SHADOW</span></div>{grouped.map((pair, index) => <div className="comparison-row" key={`${pair.productId}-${index}`}><div className="decision-product"><b>{productName(pair.productId)}</b><small>{dateLabel(pair.createdAt)} · {shortId(pair.rules?.evaluation_id ?? pair.jev?.evaluation_id ?? '')}</small></div><DecisionCell row={pair.rules} /><DecisionCell row={pair.jev} shadow /></div>)}</div>}
          </section>

          <section className="panel suggestions-panel">
            <div className="result-header"><div><span className="result-overline">04 / HUMAN FEEDBACK</span><h3>Suggestions</h3><p>Only production rules create these. Record feedback to shape later evaluations.</p></div><span className="live-tag">{suggestions.data?.length ?? 0} TOTAL</span></div>
            {suggestions.isLoading ? <div className="empty-state">Loading suggestions…</div> : suggestions.isError ? <div className="empty-state error-text">{messageOf(suggestions.error)}</div> : !suggestions.data?.length ? <div className="empty-state">No suggestions for this customer yet.</div> : <div className="suggestion-list">{suggestions.data.map((suggestion) => <SuggestionCard key={suggestion.id} item={suggestion} productName={productName} onFeedback={(verb) => mutation.mutate({ path: `/suggestions/${suggestion.id}/${verb}`, method: 'POST' })} disabled={working} />)}</div>}
          </section>

          <footer className="page-footer"><span>SMARTREPLENISH <b>·</b> DECISION LAB</span><span>Data persists in your local PostgreSQL database</span></footer>
        </div>
      </section>
    </main>
  )
}

function Metric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return <div className="metric-row"><div><b>{label}</b><small>{detail}</small></div><strong>{value}</strong></div>
}

function DecisionCell({ row, shadow = false }: { row?: DecisionLog; shadow?: boolean }) {
  if (!row) return <div className="decision-cell missing"><span>NO PAIRED RESULT</span></div>
  if (row.error) return <div className="decision-cell decision-error"><span className="decision-label">{shadow ? 'JEV ERROR' : 'ENGINE ERROR'}</span><b>{row.error}</b><small>{row.latency_ms} ms</small></div>
  const result = row.decision_result
  return <div className={`decision-cell ${shadow ? 'jev-cell' : ''}`}><span className="decision-label">{result?.provider ?? row.engine} {result?.model ? `· ${result.model}` : ''}</span><b>{result?.action ?? row.executed_action}</b><small>{percent(result?.confidence)} confidence{result?.probability == null ? '' : ` · ${percent(result.probability)} choice probability`} · {row.latency_ms} ms</small><small className="executed-label">{shadow ? 'Shadow only · never executed' : `Policy executed · ${row.executed_action}`}</small></div>
}

function SuggestionCard({ item, productName, onFeedback, disabled }: { item: Suggestion; productName: (id: string) => string; onFeedback: (verb: string) => void; disabled: boolean }) {
  const resolved = item.status !== 'PENDING'
  return <div className="suggestion-row"><div className="suggestion-mark">✳</div><div className="suggestion-copy"><b>{item.message || item.kind.replaceAll('_', ' ')}</b><small>{item.items.map((product) => productName(product.product_id)).join(' · ')} <span>· {dateLabel(item.created_at)}</span></small></div><span className={`suggestion-status ${resolved ? 'resolved' : ''}`}>{item.status}</span>{!resolved && <div className="suggestion-actions"><button disabled={disabled} onClick={() => onFeedback('shown')}>Shown</button><button disabled={disabled} onClick={() => onFeedback('accept')}>Accept</button><button disabled={disabled} onClick={() => onFeedback('dismiss')}>Dismiss</button></div>}</div>
}

export default App
