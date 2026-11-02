import { useEffect, useState, type FormEvent } from 'react'
import { errorMessage, listChecks, openCheck, type Check } from './api.ts'
import { time, yen } from './format.ts'
import { StatusBadge } from './StatusBadge.tsx'

type Props = {
  onSelect: (checkId: number) => void
}

// CheckList はテーブル一覧の画面。席にいるお客様の伝票を並べ、新しい伝票を開く。
export function CheckList({ onSelect }: Props) {
  const [checks, setChecks] = useState<Check[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    // 応答が返る前に画面を離れたとき、古い応答で state を書き換えないようにする。
    let ignore = false
    // 会計済みでも退店するまでは席にいるため、open と paid の両方を並べる。
    Promise.all([listChecks('open'), listChecks('paid')])
      .then(([open, paid]) => {
        if (!ignore) setChecks([...open, ...paid].sort((a, b) => a.tableNo - b.tableNo))
      })
      .catch((e: unknown) => {
        if (!ignore) setLoadError(errorMessage(e))
      })
    return () => {
      ignore = true
    }
  }, [])

  return (
    <main>
      <h1>テーブル一覧</h1>

      <OpenCheckForm onOpened={onSelect} />

      {loadError && <p className="error">{loadError}</p>}
      {checks === null && !loadError && <p>読み込み中…</p>}
      {checks?.length === 0 && <p className="muted">利用中のテーブルはありません</p>}

      {checks && checks.length > 0 && (
        <ul className="check-list">
          {checks.map((c) => (
            <li key={c.id}>
              <button type="button" className="check-card" onClick={() => onSelect(c.id)}>
                <span className="table-no">テーブル {c.tableNo}</span>
                <StatusBadge status={c.status} />
                <span className="muted">{time(c.openedAt)} 入店</span>
                <span className="amount">{yen(c.totalYen)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

// OpenCheckForm はテーブル番号を受け取り、伝票を開く。開けたら伝票の画面へ進む。
function OpenCheckForm({ onOpened }: { onOpened: (checkId: number) => void }) {
  const [tableNo, setTableNo] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const c = await openCheck(Number(tableNo))
      onOpened(c.id)
    } catch (e) {
      // 利用中の伝票があるテーブル (409) などは、バックエンドの文面をそのまま見せる。
      setError(errorMessage(e))
      setBusy(false)
    }
  }

  return (
    <form className="open-form" onSubmit={handleSubmit}>
      <label>
        テーブル番号
        <input
          type="number"
          min={1}
          required
          value={tableNo}
          onChange={(e) => setTableNo(e.target.value)}
        />
      </label>
      <button type="submit" className="primary" disabled={busy}>
        伝票を開く
      </button>
      {error && <p className="error">{error}</p>}
    </form>
  )
}
