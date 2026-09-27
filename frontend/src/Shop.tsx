import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { askKind, fetchProducts, json, messageOf, productIcon, request, type CartLine, type Evaluation, type Prediction, type Product, type Suggestion } from './api'

type Props = { userId: string; offline: boolean }

function Shop({ userId, offline }: Props) {
  const queryClient = useQueryClient()
  const [category, setCategory] = useState('All')
  const [search, setSearch] = useState('')
  const [toast, setToast] = useState('')
  const shownIds = useRef(new Set<string>())
  const evaluatedFor = useRef('')

  const products = useQuery({ queryKey: ['products'], queryFn: fetchProducts })
  const cart = useQuery({ queryKey: ['cart', userId], queryFn: () => request<CartLine[]>(`/users/${userId}/cart`) })
  const suggestions = useQuery({ queryKey: ['suggestions', userId], queryFn: () => request<Suggestion[]>(`/users/${userId}/suggestions?limit=50`) })
  const predictions = useQuery({ queryKey: ['predictions', userId], queryFn: () => request<Prediction[]>(`/users/${userId}/replenishment`) })

  const refresh = () => Promise.all(['cart', 'suggestions', 'predictions', 'history', 'metrics'].map((key) => queryClient.invalidateQueries({ queryKey: [key, userId] })))
  const fail = (error: unknown) => setToast(messageOf(error))

  // Opening the app asks the backend which purchased items are running low.
  const check = useMutation({
    mutationFn: () => request<Evaluation>(`/users/${userId}/decisions/evaluate`, json('POST', {})),
    onSuccess: refresh,
    onError: fail,
  })
  useEffect(() => {
    if (offline || evaluatedFor.current === userId) return
    evaluatedFor.current = userId
    check.mutate()
  }, [userId, offline])

  const answer = useMutation({
    mutationFn: ({ id, runningLow }: { id: string; runningLow: boolean; name: string }) => request<Suggestion>(`/suggestions/${id}/answer`, json('POST', { running_low: runningLow })),
    onSuccess: async (_, { runningLow, name }) => {
      setToast(runningLow ? `${name} added to your cart.` : `Got it — we won't ask about ${name} for a while.`)
      await refresh()
    },
    onError: fail,
  })
  const setLine = useMutation({
    mutationFn: ({ productId, quantity }: { productId: string; quantity: number }) => {
      const line = cartLines.get(productId)
      return request<CartLine>(`/users/${userId}/products/${productId}/state`, json('PUT', { in_cart: quantity > 0, in_list: line?.in_list ?? false, cart_quantity: quantity }))
    },
    onSuccess: refresh,
    onError: fail,
  })
  const order = useMutation({
    mutationFn: (lines: CartLine[]) => request('/purchases', json('POST', {
      user_id: userId,
      // A small offset keeps a fast client clock from producing a future timestamp.
      purchased_at: new Date(Date.now() - 5_000).toISOString(),
      source: 'app',
      total_amount: 0,
      items: lines.map((line) => ({ product_id: line.product_id, quantity: line.cart_quantity, unit_price: 0 })),
    }, { 'Idempotency-Key': crypto.randomUUID() })),
    onSuccess: async () => { setToast('Demo order recorded. Your purchase history now includes it.'); await refresh() },
    onError: fail,
  })

  const catalogue = products.data ?? []
  const byId = useMemo(() => new Map(catalogue.map((product) => [product.id, product])), [catalogue])
  const cartLines = useMemo(() => new Map((cart.data ?? []).filter((line) => line.in_cart).map((line) => [line.product_id, line])), [cart.data])
  const predictionById = useMemo(() => new Map((predictions.data ?? []).map((item) => [item.product_id, item])), [predictions.data])
  const questions = (suggestions.data ?? []).filter((item) => item.kind === askKind && item.status === 'pending')
  const categories = ['All', ...new Set(catalogue.map((product) => product.category))]
  const visible = catalogue.filter((product) => (category === 'All' || product.category === category) && product.name.toLowerCase().includes(search.trim().toLowerCase()))
  const lines = [...cartLines.values()]
  const units = lines.reduce((sum, line) => sum + line.cart_quantity, 0)
  const busy = answer.isPending || setLine.isPending || order.isPending

  // An impression is recorded only once a question is actually rendered.
  useEffect(() => {
    for (const question of questions) {
      if (shownIds.current.has(question.id)) continue
      shownIds.current.add(question.id)
      request(`/suggestions/${question.id}/shown`, { method: 'POST' }).catch(() => shownIds.current.delete(question.id))
    }
  }, [questions])

  return (
    <div className="shop">
      <section className="shop-hero">
        <div>
          <span className="shop-kicker">SMARTMART · DELIVERY IN 2 HOURS</span>
          <h1>Good to see you again<span>.</span></h1>
          <p>We keep an eye on the things you buy regularly. When something looks like it's running low, we'll ask — say yes and it goes straight into your cart.</p>
        </div>
        <button className="button primary pantry-button" disabled={check.isPending || offline} onClick={() => check.mutate()}>
          {check.isPending ? 'Checking your pantry…' : 'Check what’s running low'}
        </button>
      </section>

      {toast && <div className={`notice ${/fail|invalid|not found|conflict|offline/i.test(toast) ? 'notice-error' : ''}`} role="status"><span>{/fail|invalid|not found|conflict|offline/i.test(toast) ? '!' : '✓'}</span>{toast}<button aria-label="Dismiss" onClick={() => setToast('')}>×</button></div>}

      {questions.length > 0 && (
        <section className="running-low" aria-label="Running low">
          <div className="running-low-head"><span className="running-low-badge">RUNNING LOW?</span><h2>Time to restock?</h2><p>Based on how often you buy these.</p></div>
          <div className="question-list">
            {questions.map((question) => {
              const productId = question.items[0]?.product_id ?? ''
              const product = byId.get(productId)
              const prediction = predictionById.get(productId)
              const quantity = Math.min(999, Math.max(1, Math.round(prediction?.median_quantity ?? 1)))
              const name = product?.name ?? 'this item'
              return (
                <article className="question-card" key={question.id}>
                  <div className="question-icon">{productIcon(product)}</div>
                  <div className="question-copy">
                    <b>{question.message}</b>
                    <small>{prediction ? `You usually buy ${name.toLowerCase()} every ${Math.round(prediction.median_repurchase_days)} days · last bought ${Math.round(prediction.days_since_last_purchase)} days ago` : product?.brand}</small>
                  </div>
                  <div className="question-actions">
                    <button className="answer-yes" disabled={busy} onClick={() => answer.mutate({ id: question.id, runningLow: true, name })}>Yes, add {quantity}</button>
                    <button className="answer-no" disabled={busy} onClick={() => answer.mutate({ id: question.id, runningLow: false, name })}>Not yet</button>
                  </div>
                </article>
              )
            })}
          </div>
        </section>
      )}

      <div className="shop-layout">
        <section className="catalogue">
          <div className="catalogue-tools">
            <input className="search" type="search" placeholder="Search products" value={search} onChange={(event) => setSearch(event.target.value)} aria-label="Search products" />
            <div className="chips" role="tablist" aria-label="Categories">
              {categories.map((name) => <button key={name} role="tab" aria-selected={category === name} className={`chip ${category === name ? 'active' : ''}`} onClick={() => setCategory(name)}>{name}</button>)}
            </div>
          </div>
          {products.isLoading ? <div className="empty-state">Loading the shelves…</div>
            : products.isError ? <div className="empty-state error-text">{messageOf(products.error)}</div>
            : visible.length === 0 ? <div className="empty-state">No products match. Run <code>make seed</code> or add products in the Decision lab.</div>
            : <div className="product-grid">{visible.map((product) => <ProductCard key={product.id} product={product} line={cartLines.get(product.id)} prediction={predictionById.get(product.id)} disabled={busy} onQuantity={(quantity) => setLine.mutate({ productId: product.id, quantity })} />)}</div>}
        </section>

        <aside className="cart panel" aria-label="Cart">
          <div className="cart-head"><h2>Your cart</h2><span className="live-tag">{units} {units === 1 ? 'ITEM' : 'ITEMS'}</span></div>
          {lines.length === 0 ? <div className="cart-empty">Your cart is empty. Items you confirm as running low appear here automatically.</div> : (
            <ul className="cart-lines">
              {lines.map((line) => {
                const product = byId.get(line.product_id)
                return (
                  <li key={line.product_id} className={line.auto_added ? 'auto' : ''}>
                    <span className="cart-icon">{productIcon(product)}</span>
                    <div className="cart-copy">
                      <b>{product?.name ?? line.product_id}</b>
                      {line.auto_added ? <small className="auto-tag">⚡ Added because you're running low</small> : <small>{product?.unit}</small>}
                    </div>
                    <Stepper value={line.cart_quantity} disabled={busy} onChange={(quantity) => setLine.mutate({ productId: line.product_id, quantity })} />
                  </li>
                )
              })}
            </ul>
          )}
          <button className="button primary checkout" disabled={busy || lines.length === 0 || offline} onClick={() => order.mutate(lines)}>{order.isPending ? 'Recording…' : 'Place demo order'}</button>
          <p className="cart-note">Demo only: no payment is taken. Ordering records a purchase so the next prediction uses it.</p>
        </aside>
      </div>
    </div>
  )
}

function ProductCard({ product, line, prediction, disabled, onQuantity }: { product: Product; line?: CartLine; prediction?: Prediction; disabled: boolean; onQuantity: (quantity: number) => void }) {
  const low = prediction?.eligible && prediction.estimated_days_remaining != null && prediction.estimated_days_remaining <= 2
  return (
    <article className={`product-card ${product.available ? '' : 'sold-out'}`}>
      <div className="product-art">{productIcon(product)}{low && <span className="low-flag">Running low</span>}{line?.auto_added && <span className="auto-flag">⚡ Auto-added</span>}</div>
      <div className="product-body">
        <small>{product.brand || product.category}</small>
        <b>{product.name}</b>
        <span className="product-meta">{prediction ? `Bought ${prediction.total_purchase_count}× before` : `Per ${product.unit}`}</span>
      </div>
      <div className="product-actions">
        {!product.available ? <span className="sold-out-label">Out of stock</span>
          : line ? <Stepper value={line.cart_quantity} disabled={disabled} onChange={onQuantity} />
          : <button className="add-button" disabled={disabled} onClick={() => onQuantity(1)}>Add</button>}
      </div>
    </article>
  )
}

function Stepper({ value, disabled, onChange }: { value: number; disabled: boolean; onChange: (value: number) => void }) {
  return (
    <div className="stepper">
      <button aria-label={value === 1 ? 'Remove' : 'Decrease'} disabled={disabled} onClick={() => onChange(value - 1)}>{value === 1 ? '🗑' : '−'}</button>
      <span>{value}</span>
      <button aria-label="Increase" disabled={disabled || value >= 999} onClick={() => onChange(value + 1)}>+</button>
    </div>
  )
}

export default Shop
