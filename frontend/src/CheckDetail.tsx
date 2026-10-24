import { useEffect, useState } from 'react'
import {
  addOrder,
  closeCheck,
  errorMessage,
  getCheck,
  listMenuItems,
  payCheck,
  type Check,
  type MenuItem,
} from './api.ts'
import { time, yen } from './format.ts'
import { StatusBadge } from './StatusBadge.tsx'

type Props = {
  checkId: number
  onBack: () => void
}

// Cart は注文する前の品目と数量。キーはメニューの識別子。
type Cart = Record<number, number>

// CheckDetail は 1 枚の伝票の画面。注文の履歴と合計を見せ、注文・会計・退店を行う。
export function CheckDetail({ checkId, onBack }: Props) {
  const [check, setCheck] = useState<Check | null>(null)
  const [menu, setMenu] = useState<MenuItem[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [cart, setCart] = useState<Cart>({})
  // 増やすと伝票を読み直す。
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let ignore = false
    getCheck(checkId)
      .then((c) => {
        if (!ignore) setCheck(c)
      })
      .catch((e: unknown) => {
        if (!ignore) setLoadError(errorMessage(e))
      })
    return () => {
      ignore = true
    }
  }, [checkId, version])

  useEffect(() => {
    let ignore = false
    listMenuItems()
      .then((m) => {
        if (!ignore) setMenu(m)
      })
      .catch((e: unknown) => {
        if (!ignore) setLoadError(errorMessage(e))
      })
    return () => {
      ignore = true
    }
  }, [])

  // run は伝票を書き換える操作を 1 つ行う。実行中はボタンを押せなくし、二重送信を防ぐ。
  async function run(action: () => Promise<void>) {
    setBusy(true)
    setActionError(null)
    try {
      await action()
    } catch (e) {
      setActionError(errorMessage(e))
      // 別の端末で会計されたなど、画面の表示が古い可能性があるため読み直す。
      setVersion((v) => v + 1)
    } finally {
      setBusy(false)
    }
  }

  function submitOrder() {
    const items = Object.entries(cart).map(([id, quantity]) => ({ menuItemId: Number(id), quantity }))
    return run(async () => {
      await addOrder(checkId, items)
      setCart({})
      setVersion((v) => v + 1)
    })
  }

  function pay(total: number) {
    // 会計は取り消せないため、金額を見せて確かめる。
    if (!window.confirm(`${yen(total)} を会計します。よろしいですか？`)) return
    return run(async () => {
      setCheck(await payCheck(checkId))
    })
  }

  function close() {
    return run(async () => {
      await closeCheck(checkId)
      onBack()
    })
  }

  if (loadError) {
    return (
      <main>
        <BackButton onClick={onBack} />
        <p className="error">{loadError}</p>
      </main>
    )
  }
  if (check === null) {
    return (
      <main>
        <BackButton onClick={onBack} />
        <p>読み込み中…</p>
      </main>
    )
  }

  return (
    <main>
      <BackButton onClick={onBack} />
      <h1>
        テーブル {check.tableNo} <StatusBadge status={check.status} />
      </h1>
      <p className="muted">{time(check.openedAt)} 入店</p>

      <section>
        <h2>注文の履歴</h2>
        <OrderHistory check={check} />
      </section>

      {actionError && <p className="error">{actionError}</p>}

      {check.status === 'open' && menu && (
        <section>
          <h2>注文を追加</h2>
          <MenuPicker menu={menu} cart={cart} onChange={setCart} />
          <CartSummary menu={menu} cart={cart} busy={busy} onSubmit={submitOrder} />
        </section>
      )}

      <div className="actions">
        {check.status === 'open' && (
          <button type="button" className="primary" disabled={busy} onClick={() => pay(check.totalYen)}>
            会計する
          </button>
        )}
        {check.status === 'paid' && (
          <button type="button" className="primary" disabled={busy} onClick={close}>
            伝票を閉じる（退店）
          </button>
        )}
      </div>
    </main>
  )
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" className="link" onClick={onClick}>
      ← テーブル一覧へ
    </button>
  )
}

function OrderHistory({ check }: { check: Check }) {
  if (check.orders.length === 0) {
    return <p className="muted">まだ注文はありません</p>
  }
  return (
    <table className="order-table">
      <tbody>
        {check.orders.map((o) =>
          o.items.map((it, i) => (
            <tr key={it.id} className={i === 0 ? 'order-first' : undefined}>
              <td className="muted">{i === 0 ? time(o.orderedAt) : ''}</td>
              <td>{it.name}</td>
              <td className="num">
                {yen(it.priceYen)} × {it.quantity}
              </td>
              <td className="num">{yen(it.subtotalYen)}</td>
            </tr>
          )),
        )}
      </tbody>
      <tfoot>
        <tr>
          <th colSpan={3}>合計</th>
          <td className="num amount">{yen(check.totalYen)}</td>
        </tr>
      </tfoot>
    </table>
  )
}

const categoryLabels: Record<MenuItem['category'], string> = { food: '料理', drink: '飲み物' }

function MenuPicker({ menu, cart, onChange }: { menu: MenuItem[]; cart: Cart; onChange: (cart: Cart) => void }) {
  function setQuantity(id: number, quantity: number) {
    const next = { ...cart }
    if (quantity > 0) {
      next[id] = quantity
    } else {
      delete next[id]
    }
    onChange(next)
  }

  return (
    <>
      {(['food', 'drink'] as const).map((category) => (
        <div key={category} className="menu-group">
          <h3>{categoryLabels[category]}</h3>
          <ul className="menu-list">
            {menu
              .filter((m) => m.category === category)
              .map((m) => {
                const quantity = cart[m.id] ?? 0
                return (
                  <li key={m.id} className={m.available ? undefined : 'sold-out'}>
                    <span>{m.name}</span>
                    <span className="num muted">{m.available ? yen(m.priceYen) : '品切れ'}</span>
                    <span className="stepper">
                      <button
                        type="button"
                        aria-label={`${m.name}を減らす`}
                        disabled={quantity === 0}
                        onClick={() => setQuantity(m.id, quantity - 1)}
                      >
                        −
                      </button>
                      <span className="num">{quantity}</span>
                      <button
                        type="button"
                        aria-label={`${m.name}を増やす`}
                        disabled={!m.available}
                        onClick={() => setQuantity(m.id, quantity + 1)}
                      >
                        ＋
                      </button>
                    </span>
                  </li>
                )
              })}
          </ul>
        </div>
      ))}
    </>
  )
}

function CartSummary({
  menu,
  cart,
  busy,
  onSubmit,
}: {
  menu: MenuItem[]
  cart: Cart
  busy: boolean
  onSubmit: () => void
}) {
  const count = Object.values(cart).reduce((sum, q) => sum + q, 0)
  // 画面で見せる目安。確定する金額はバックエンドが注文時点のメニューから計算する。
  const estimate = menu.reduce((sum, m) => sum + m.priceYen * (cart[m.id] ?? 0), 0)

  return (
    <div className="cart">
      <span>
        {count} 点 / {yen(estimate)}
      </span>
      <button type="button" className="primary" disabled={busy || count === 0} onClick={onSubmit}>
        注文する
      </button>
    </div>
  )
}
