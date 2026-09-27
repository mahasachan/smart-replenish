import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { askKind, dateLabel, fetchProducts, json, messageOf, percent, request, shortId, type DecisionLog, type Evaluation, type Metrics, type Prediction, type Product, type Rate, type Suggestion, type User } from './api'

type Props = { userId: string; offline: boolean; onUserCreated: (id: string) => void }
type Pair = { productId: string; evaluationId: string; createdAt: string; rules?: DecisionLog; jev?: DecisionLog }

const actions = ['ASK_IF_RUNNING_LOW', 'SUGGEST_NOW', 'SUGGEST_BUNDLE', 'ADD_TO_SMART_LIST_SUGGESTION', 'WAIT', 'DO_NOTHING']
const label = (value: string) => value.replaceAll('_', ' ').toLowerCase()
const days = (value: number | null | undefined) => value == null ? '—' : `${value.toFixed(1)}d`
const rateDetail = (rate: Rate | undefined, noun: string) => `${rate?.numerator ?? 0} / ${rate?.denominator ?? 0} ${noun}`

function Lab({ userId, offline, onUserCreated }: Props) {
  const queryClient = useQueryClient()
  const [productId, setProductId] = useState('')
  const [notice, setNotice] = useState('')
  const [evaluation, setEvaluation] = useState<Evaluation | null>(null)

  const products = useQuery({ queryKey: ['products'], queryFn: fetchProducts })
  const predictions = useQuery({ queryKey: ['predictions', userId], queryFn: () => request<Prediction[]>(`/users/${userId}/replenishment`) })
  const suggestions = useQuery({ queryKey: ['suggestions', userId], queryFn: () => request<Suggestion[]>(`/users/${userId}/suggestions?limit=50`) })
  const history = useQuery({ queryKey: ['history', userId], queryFn: () => request<DecisionLog[]>(`/users/${userId}/decision-history?limit=100`) })
  const metrics = useQuery({ queryKey: ['metrics', userId], queryFn: () => request<Metrics>(`/users/${userId}/evaluation-metrics`) })

  const invalidateUserData = () => Promise.all(['predictions', 'suggestions', 'history', 'metrics', 'cart'].map((key) => queryClient.invalidateQueries({ queryKey: [key, userId] })))
  const mutation = useMutation({
    mutationFn: ({ path, method, body, headers }: { path: string; method: string; body?: unknown; headers?: Record<string, string> }) =>
      request<unknown>(path, body === undefined ? { method } : json(method, body, headers)),
    onSuccess: async () => { setEvaluation(null); setNotice('Saved to the backend.'); await Promise.all([invalidateUserData(), queryClient.invalidateQueries({ queryKey: ['products'] })]) },
    onError: (error) => setNotice(messageOf(error)),
  })
  const evaluate = useMutation({
    mutationFn: () => request<Evaluation>(`/users/${userId}/decisions/evaluate`, json('POST', {})),
    onSuccess: async (result) => {
      setEvaluation(result)
      setNotice(`Evaluation ${shortId(result.evaluation_id)} completed.`)
      await invalidateUserData()
    },
    onError: (error) => setNotice(messageOf(error)),
  })
  const createUser = useMutation({
    mutationFn: (payload: Omit<User, 'id'>) => request<User>('/users', json('POST', payload)),
    onSuccess: (user) => { onUserCreated(user.id); setEvaluation(null); setNotice(`User ${shortId(user.id)} created.`) },
    onError: (error) => setNotice(messageOf(error)),
  })
  const createProduct = useMutation({
    mutationFn: (payload: Omit<Product, 'id'>) => request<Product>('/products', json('POST', payload)),
    onSuccess: async (product) => {
      setProductId(product.id)
      setEvaluation(null)
      setNotice(`Product ${product.name} created.`)
      await queryClient.invalidateQueries({ queryKey: ['products'] })
    },
    onError: (error) => setNotice(messageOf(error)),
  })

  const catalogue = products.data ?? []
  const productName = (id: string) => catalogue.find((product) => product.id === id)?.name ?? shortId(id)
  const pairs = useMemo(() => {
    const map = new Map<string, Pair>()
    for (const row of history.data ?? []) {
      const key = `${row.evaluation_id}:${row.product_id}`
      const pair = map.get(key) ?? { productId: row.product_id, evaluationId: row.evaluation_id, createdAt: row.created_at }
      if (row.shadow) pair.jev = row
      else pair.rules = row
      map.set(key, pair)
    }
    return [...map.values()]
  }, [history.data])
  // History is newest first, so the first pair seen per product is its latest evaluation.
  const latest = useMemo(() => {
    const seen = new Map<string, Pair>()
    for (const pair of pairs) if (!seen.has(pair.productId)) seen.set(pair.productId, pair)
    return [...seen.values()]
  }, [pairs])

  const submit = (event: FormEvent<HTMLFormElement>, action: () => void) => { event.preventDefault(); action() }
  const chooseProduct = productId || catalogue[0]?.id || ''
  const working = mutation.isPending || evaluate.isPending || createUser.isPending || createProduct.isPending
  const running = metrics.data?.running_low

  return (
    <div className="page-content">
      <section className="hero">
        <div>
          <div className="eyebrow"><span className="eyebrow-line" /> REPLENISHMENT WORKBENCH</div>
          <h1>Decision lab<span>.</span></h1>
          <p>Shape a customer’s purchase history, run an evaluation, and inspect every score: the prediction, the rule decision and policy outcome, and Jev’s shadow choice with its probability for each action.</p>
        </div>
        <div className="hero-meta"><span className="pulse" /> LIVE WORKSPACE <span className="meta-sep">·</span> LOCAL DATA</div>
      </section>

      <section className="control-strip panel">
        <div className="control-label"><span className="step-number">01</span><div><b>Focus product</b><small>Used by the forms and controls below</small></div></div>
        <div className="control-fields">
          <label className="field compact"><span>FOCUS PRODUCT</span><select value={chooseProduct} onChange={(event) => setProductId(event.target.value)}>{catalogue.map((product) => <option key={product.id} value={product.id}>{product.name} · {product.sku}</option>)}</select></label>
        </div>
        <button className="button primary evaluate-button" disabled={working || offline} onClick={() => evaluate.mutate()}><span className="button-icon">◈</span>{evaluate.isPending ? 'Evaluating…' : 'Run evaluation'}<span className="button-arrow">↗</span></button>
      </section>

      {notice && <div className={`notice ${/failed|invalid|not found|offline|conflict/i.test(notice) ? 'notice-error' : ''}`}><span>{/completed|created|Saved/.test(notice) ? '✓' : '!'}</span>{notice}<button aria-label="Dismiss" onClick={() => setNotice('')}>×</button></div>}
      {evaluation?.shadow_warning && <div className="warning-banner"><span>!</span><div><b>Jev shadow comparison incomplete</b><small>{evaluation.shadow_warning}</small></div></div>}
      {evaluation && !evaluation.shadow_enabled && <div className="warning-banner muted-warning"><span>i</span><div><b>Jev is disabled</b><small>Add OPENROUTER_API_KEY to backend/.env and restart the backend to compare shadow decisions.</small></div></div>}

      <div className="section-heading"><div><span className="section-kicker">02 / SCORES</span><h2>Score inspector</h2></div><span className="section-note">Latest evaluation per product</span></div>
      <section className="score-grid">
        {history.isLoading ? <div className="panel empty-state">Loading decisions…</div>
          : history.isError ? <div className="panel empty-state error-text">{messageOf(history.error)}</div>
          : latest.length === 0 ? <div className="panel empty-state">No decisions yet. Add purchase history and run an evaluation.</div>
          : latest.map((pair) => <ScoreCard key={`${pair.evaluationId}:${pair.productId}`} pair={pair} name={productName(pair.productId)} />)}
      </section>

      <div className="section-heading results-heading"><div><span className="section-kicker">03 / QUALITY</span><h2>Observed outcomes</h2></div><span className="section-note">Descriptive rates, not causal lift</span></div>
      <section className="results-grid">
        <article className="panel metrics-panel">
          <div className="result-header"><div><span className="result-overline">RUNNING-LOW FEEDBACK</span><h3>Was the prediction right?</h3></div><span className="live-tag">{running?.questions_shown ?? 0} ASKED</span></div>
          {metrics.isError ? <div className="empty-state error-text">{messageOf(metrics.error)}</div> : <div className="metric-list">
            <MetricRow label="Confirmed running low" value={percent(running?.confirmed_prediction_precision.rate)} detail={`${rateDetail(running?.confirmed_prediction_precision, 'shown questions')} · prediction precision`} />
            <MetricRow label="Answered not yet" value={percent(running?.denied.rate)} detail={rateDetail(running?.denied, 'shown questions')} />
            <MetricRow label="Bought within 7 days of yes" value={percent(running?.purchase_within_7d_after_confirm.rate)} detail={rateDetail(running?.purchase_within_7d_after_confirm, 'mature confirmations')} />
          </div>}
          <p className="metric-footnote">Unanswered questions stay in the denominator. “Yes” adds the item to the cart; it never places an order.</p>
        </article>
        <article className="panel metrics-panel">
          <div className="result-header"><div><span className="result-overline">RULES VS JEV</span><h3>Shadow comparison</h3></div><span className="metrics-icon">⌁</span></div>
          {metrics.isError ? <div className="empty-state error-text">{messageOf(metrics.error)}</div> : <div className="metric-list">
            <MetricRow label="Action agreement" value={percent(metrics.data?.shadow_action_agreement.rate)} detail={rateDetail(metrics.data?.shadow_action_agreement, 'valid pairs')} />
            <MetricRow label="Pair coverage" value={percent(metrics.data?.shadow_valid_pair_coverage.rate)} detail={rateDetail(metrics.data?.shadow_valid_pair_coverage, 'production decisions')} />
            {Object.entries(metrics.data?.engines ?? {}).map(([engine, stats]) => <MetricRow key={engine} label={`${engine} engine`} value={`${Math.round(stats.average_latency_ms)} ms`} detail={`${stats.decisions} decisions · ${stats.errors} errors${stats.known_cost_usd == null ? '' : ` · $${stats.known_cost_usd.toFixed(4)}`}`} />)}
          </div>}
          <p className="metric-footnote">Agreement compares selected actions; it does not measure correctness. Jev never executes.</p>
        </article>
      </section>

      <section className="panel comparison-panel">
        <div className="result-header comparison-head"><div><span className="result-overline">04 / DECISION TRACE</span><h3>Rules vs Jev history</h3><p>Matched by evaluation and product so every comparison uses the same context.</p></div><div className="engine-legend"><span><i className="legend-rule" /> PRODUCTION RULES</span><span><i className="legend-jev" /> JEV SHADOW</span></div></div>
        {pairs.length === 0 ? <div className="empty-state">No decisions yet.</div> : <div className="comparison-table"><div className="comparison-columns"><span>PRODUCT / EVALUATION</span><span>RULE ENGINE</span><span>JEV · SHADOW</span></div>{pairs.slice(0, 12).map((pair) => <div className="comparison-row" key={`${pair.evaluationId}:${pair.productId}`}><div className="decision-product"><b>{productName(pair.productId)}</b><small>{dateLabel(pair.createdAt)} · {shortId(pair.evaluationId)}</small></div><DecisionCell row={pair.rules} /><DecisionCell row={pair.jev} shadow /></div>)}</div>}
      </section>

      <section className="panel suggestions-panel">
        <div className="result-header"><div><span className="result-overline">05 / HUMAN FEEDBACK</span><h3>Suggestions</h3><p>Only production rules create these. Running-low questions are answered in the Shop tab or here.</p></div><span className="live-tag">{suggestions.data?.length ?? 0} TOTAL</span></div>
        {suggestions.isError ? <div className="empty-state error-text">{messageOf(suggestions.error)}</div> : !suggestions.data?.length ? <div className="empty-state">No suggestions for this customer yet.</div> : <div className="suggestion-list">{suggestions.data.map((suggestion) => <SuggestionRow key={suggestion.id} item={suggestion} productName={productName} disabled={working} onFeedback={(path, body) => mutation.mutate({ path: `/suggestions/${suggestion.id}/${path}`, method: 'POST', body })} />)}</div>}
      </section>

      <div className="section-heading"><div><span className="section-kicker">06 / BUILD A SCENARIO</span><h2>Customer data</h2></div><span className="section-note">Inputs are written directly to your local database</span></div>
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
          <div className="card-heading"><div className="card-icon blue">▤</div><div><h3>Product</h3><p>Add an item to the catalogue</p></div><span className="card-index">B</span></div>
          <form onSubmit={(event) => submit(event, () => {
            const data = new FormData(event.currentTarget)
            createProduct.mutate({ sku: String(data.get('sku')), name: String(data.get('name')), category: String(data.get('category') || 'household'), brand: String(data.get('brand') || ''), unit: String(data.get('unit') || 'piece'), replenishable_score: Number(data.get('score') || 0.8), available: true })
          })}>
            <div className="form-row"><label className="form-field"><span>PRODUCT NAME</span><input name="name" required placeholder="Oat milk" /></label><label className="form-field narrow"><span>SKU</span><input name="sku" required placeholder="OAT-01" /></label></div>
            <div className="form-row"><label className="form-field"><span>CATEGORY</span><input name="category" placeholder="dairy" /></label><label className="form-field score-field"><span>REPLENISH SCORE</span><input name="score" type="number" min="0" max="1" step="0.01" defaultValue="0.85" /></label></div>
            <button className="button secondary" disabled={working}>Create product <b>＋</b></button>
          </form>
        </article>
        <article className="panel data-card purchase-card">
          <div className="card-heading"><div className="card-icon gold">↗</div><div><h3>Purchase event</h3><p>Add one dated purchase of {productName(chooseProduct)}</p></div><span className="card-index">C</span></div>
          <form onSubmit={(event) => submit(event, () => {
            const data = new FormData(event.currentTarget)
            const quantity = Number(data.get('quantity'))
            const unitPrice = Number(data.get('unit_price'))
            const purchasedAt = new Date(`${String(data.get('date'))}T12:00:00`)
            const past = new Date(Math.min(purchasedAt.getTime(), Date.now() - 5_000)).toISOString()
            mutation.mutate({ path: '/purchases', method: 'POST', body: { user_id: userId, purchased_at: past, source: 'app', total_amount: quantity * unitPrice, items: [{ product_id: chooseProduct, quantity, unit_price: unitPrice }] }, headers: { 'Idempotency-Key': crypto.randomUUID() } })
          })}>
            <div className="form-row"><label className="form-field"><span>PURCHASE DATE</span><input name="date" type="date" required defaultValue={new Date().toISOString().slice(0, 10)} /></label><label className="form-field narrow"><span>QUANTITY</span><input name="quantity" type="number" min="1" step="1" defaultValue="1" required /></label><label className="form-field narrow"><span>UNIT PRICE · MINOR</span><input name="unit_price" type="number" min="0" step="1" defaultValue="100" required /></label></div>
            <button className="button secondary" disabled={working || !chooseProduct}>Record purchase <b>＋</b></button>
          </form>
        </article>
      </section>

      <div className="section-heading state-heading"><div><span className="section-kicker">07 / TUNE CURRENT CONDITIONS</span><h2>Scenario controls</h2></div><span className="section-note">Changes affect the next evaluation</span></div>
      <section className="panel state-panel">
        <StateControl icon="⚙" title="Suggestion preference" caption="Allow this customer to receive suggestions" buttons={[['Disable', () => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { suggestion_enabled: false } })], ['Enable', () => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { suggestion_enabled: true } })]]} />
        <StateControl icon="◌" title="Notification preference" caption="Preference only; this system sends no notifications" buttons={[['Opt out', () => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { notification_enabled: false } })], ['Opt in', () => mutation.mutate({ path: `/users/${userId}`, method: 'PATCH', body: { notification_enabled: true } })]]} />
        <StateControl icon="◉" title="Product availability" caption={productName(chooseProduct)} buttons={[['Mark unavailable', () => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { available: false } })], ['Available', () => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { available: true } })]]} />
        <StateControl icon="⌁" title="Replenishability score" caption={`Change eligibility signal for ${productName(chooseProduct)}`} buttons={[['Low · 0.10', () => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { replenishable_score: 0.1 } })], ['High · 0.95', () => mutation.mutate({ path: `/products/${chooseProduct}`, method: 'PATCH', body: { replenishable_score: 0.95 } })]]} />
        <StateControl icon="☷" title="Shopping state" caption="Set cart and list membership" buttons={[['In cart', () => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: true, in_list: false, cart_quantity: 1 } })], ['On list', () => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: false, in_list: true } })], ['Clear', () => mutation.mutate({ path: `/users/${userId}/products/${chooseProduct}/state`, method: 'PUT', body: { in_cart: false, in_list: false } })]]} />
      </section>

      <section className="panel prediction-panel standalone">
        <div className="result-header"><div><span className="result-overline">08 / FORECAST</span><h3>Replenishment signals</h3></div><span className="live-tag">{predictions.data?.length ?? 0} ITEMS</span></div>
        {predictions.isError ? <div className="empty-state error-text">{messageOf(predictions.error)}</div> : !predictions.data?.length ? <div className="empty-state">No purchase history yet. Add dated purchase events to build a forecast.</div> : <div className="prediction-list">{predictions.data.map((item) => <div className="prediction-row" key={item.product_id}><div className="prediction-product"><span className="product-bullet" /><div><b>{productName(item.product_id)}</b><small>{item.total_purchase_count} purchase occasions · {item.eligible ? 'eligible' : item.reason?.replaceAll('_', ' ')}</small></div></div><div className="prediction-stats"><div><small>DAYS LEFT</small><b>{days(item.estimated_days_remaining)}</b></div><div><small>CONFIDENCE</small><b>{percent(item.prediction_confidence)}</b></div></div></div>)}</div>}
      </section>

      <footer className="page-footer"><span>SMARTREPLENISH <b>·</b> DECISION LAB</span><span>Data persists in your local PostgreSQL database</span></footer>
    </div>
  )
}

function ScoreCard({ pair, name }: { pair: Pair; name: string }) {
  const product = (pair.rules ?? pair.jev)?.decision_context.product
  const rules = pair.rules?.decision_result
  const jev = pair.jev?.decision_result
  const probabilities = jev?.metadata?.probabilities ?? {}
  const agree = rules && jev ? rules.action === jev.action : null
  return (
    <article className="panel score-card">
      <div className="score-head">
        <div><b>{name}</b><small>{dateLabel(pair.createdAt)} · {shortId(pair.evaluationId)}</small></div>
        {agree != null && <span className={`agree-tag ${agree ? 'agree' : 'disagree'}`}>{agree ? 'AGREE' : 'DISAGREE'}</span>}
      </div>
      {product && (
        <dl className="score-stats">
          <div><dt>Days left</dt><dd>{days(product.estimated_days_remaining)}</dd></div>
          <div><dt>Every</dt><dd>{product.eligible ? days(product.median_repurchase_days) : '—'}</dd></div>
          <div><dt>Regularity</dt><dd>{product.eligible ? percent(product.purchase_regularity) : '—'}</dd></div>
          <div><dt>Confidence</dt><dd>{percent(product.prediction_confidence)}</dd></div>
          <div><dt>Usual qty</dt><dd>{product.median_quantity || '—'}</dd></div>
          <div><dt>Occasions</dt><dd>{product.total_purchase_count}</dd></div>
        </dl>
      )}
      {product && !product.eligible && <p className="score-note">Ineligible · {label(product.reason ?? 'unknown')}</p>}
      {product?.running_low_snooze_hours_remaining != null && <p className="score-note">Snoozed after “not yet” · {Math.ceil(product.running_low_snooze_hours_remaining)}h left</p>}
      <div className="score-engines">
        <div className="score-engine">
          <span className="decision-label">RULES · {rules?.model ?? pair.rules?.engine ?? '—'}</span>
          <b>{pair.rules?.error ? 'error' : label(rules?.action ?? '—')}</b>
          <small>Policy: {pair.rules?.policy_result ? `${pair.rules.policy_result.allowed ? 'allowed' : 'blocked'} · ${label(pair.rules.policy_result.reason)}` : '—'}</small>
          <small>Executed: {label(pair.rules?.executed_action ?? '—')}</small>
        </div>
        <div className="score-engine jev">
          <span className="decision-label">JEV · {jev?.model ?? 'shadow'}</span>
          {!pair.jev ? <small>Not evaluated (Jev disabled or no pair)</small>
            : pair.jev.error ? <><b>error</b><small className="error-text">{pair.jev.error}</small></>
            : <>
              <b>{label(jev?.action ?? '—')}</b>
              <small>Confidence {percent(jev?.confidence)} · choice p {percent(jev?.probability)} · {pair.jev.latency_ms} ms{jev?.cost_usd == null ? '' : ` · $${jev.cost_usd.toFixed(5)}`}</small>
              <small>Policy would: {pair.jev.policy_result ? `${pair.jev.policy_result.allowed ? 'allow' : 'block'} · ${label(pair.jev.policy_result.reason)}` : '—'}</small>
            </>}
        </div>
      </div>
      {Object.keys(probabilities).length > 0 && (
        <div className="probabilities" aria-label="Jev probability per action">
          {actions.filter((action) => action in probabilities).map((action) => (
            <div key={action} className={`probability ${action === jev?.action ? 'chosen' : ''}`}>
              <span>{label(action)}</span>
              <i><em style={{ width: `${Math.round((probabilities[action] ?? 0) * 100)}%` }} /></i>
              <b>{percent(probabilities[action])}</b>
            </div>
          ))}
        </div>
      )}
    </article>
  )
}

function MetricRow({ label: title, value, detail }: { label: string; value: string; detail: string }) {
  return <div className="metric-row"><div><b>{title}</b><small>{detail}</small></div><strong>{value}</strong></div>
}

function StateControl({ icon, title, caption, buttons }: { icon: string; title: string; caption: string; buttons: [string, () => void][] }) {
  return <div className="state-control"><div className="state-icon">{icon}</div><div className="state-copy"><b>{title}</b><small>{caption}</small></div>{buttons.map(([text, onClick]) => <button key={text} className="text-action" onClick={onClick}>{text}</button>)}</div>
}

function DecisionCell({ row, shadow = false }: { row?: DecisionLog; shadow?: boolean }) {
  if (!row) return <div className="decision-cell missing"><span>NO PAIRED RESULT</span></div>
  if (row.error) return <div className="decision-cell decision-error"><span className="decision-label">{shadow ? 'JEV ERROR' : 'ENGINE ERROR'}</span><b>{row.error}</b><small>{row.latency_ms} ms</small></div>
  const result = row.decision_result
  return <div className={`decision-cell ${shadow ? 'jev-cell' : ''}`}><span className="decision-label">{result?.provider ?? row.engine} {result?.model ? `· ${result.model}` : ''}</span><b>{result?.action ?? row.executed_action}</b><small>{percent(result?.confidence)} confidence{result?.probability == null ? '' : ` · ${percent(result.probability)} choice probability`} · {row.latency_ms} ms</small><small className="executed-label">{shadow ? 'Shadow only · never executed' : `Policy executed · ${row.executed_action}`}</small></div>
}

function SuggestionRow({ item, productName, onFeedback, disabled }: { item: Suggestion; productName: (id: string) => string; onFeedback: (path: string, body?: unknown) => void; disabled: boolean }) {
  const pending = item.status === 'pending'
  const ask = item.kind === askKind
  return (
    <div className="suggestion-row">
      <div className="suggestion-mark">{ask ? '?' : '✳'}</div>
      <div className="suggestion-copy"><b>{item.message || label(item.kind)}</b><small>{item.items.map((product) => productName(product.product_id)).join(' · ')} <span>· {label(item.kind)} · {dateLabel(item.created_at)}</span></small></div>
      <span className={`suggestion-status ${pending ? '' : 'resolved'}`}>{item.status.toUpperCase()}</span>
      {pending && <div className="suggestion-actions">
        <button disabled={disabled} onClick={() => onFeedback('shown')}>Shown</button>
        {ask ? <>
          <button disabled={disabled} onClick={() => onFeedback('answer', { running_low: true })}>Yes → cart</button>
          <button disabled={disabled} onClick={() => onFeedback('answer', { running_low: false })}>Not yet</button>
        </> : <>
          <button disabled={disabled} onClick={() => onFeedback('accept')}>Accept</button>
          <button disabled={disabled} onClick={() => onFeedback('dismiss')}>Dismiss</button>
        </>}
      </div>}
    </div>
  )
}

export default Lab
