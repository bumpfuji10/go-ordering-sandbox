// API の呼び出しと、その応答の形。
//
// 型はバックエンドの menu/handler.go と order/handler.go の JSON に合わせる。
// 金額はすべて円単位の整数で、合計や小計はバックエンドが計算した値をそのまま使う。

export type Category = 'food' | 'drink'

export type MenuItem = {
  id: number
  name: string
  category: Category
  priceYen: number
  available: boolean
}

// 伝票は open (利用中) -> paid (会計済) -> closed (退店済) の順に進む。
export type CheckStatus = 'open' | 'paid' | 'closed'

export type OrderItem = {
  id: number
  menuItemId: number
  name: string
  priceYen: number
  quantity: number
  subtotalYen: number
}

export type Order = {
  id: number
  orderedAt: string
  items: OrderItem[]
  totalYen: number
}

export type Check = {
  id: number
  tableNo: number
  status: CheckStatus
  openedAt: string
  paidAt: string | null
  closedAt: string | null
  orders: Order[]
  totalYen: number
}

// 注文で頼む 1 品目。単価は送らない（バックエンドがメニューから写し取る）。
export type OrderLine = {
  menuItemId: number
  quantity: number
}

// ApiError は API がエラーを返したか、API に届かなかったことを表す。
//
// message はバックエンドが返した文面で、そのまま利用者に見せてよい。
export class ApiError extends Error {
  // 0 は API に届かなかったことを表す。
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// errorMessage は catch したエラーから、利用者に見せる文面を取り出す。
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : '予期しないエラーが発生しました'
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'サーバーに接続できません')
  }

  if (!res.ok) {
    // エラーは {"error": "メッセージ"} の形で返る。中継の失敗などで
    // JSON でない応答が来ることもあるため、読めなければ状態コードで伝える。
    const body: unknown = await res.json().catch(() => null)
    const message =
      typeof body === 'object' && body !== null && 'error' in body && typeof body.error === 'string'
        ? body.error
        : `要求に失敗しました (${res.status})`
    throw new ApiError(res.status, message)
  }
  return (await res.json()) as T
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function listMenuItems(): Promise<MenuItem[]> {
  return request('/api/menu-items')
}

export function listChecks(status: CheckStatus): Promise<Check[]> {
  return request(`/api/checks?status=${status}`)
}

export function getCheck(id: number): Promise<Check> {
  return request(`/api/checks/${id}`)
}

export function openCheck(tableNo: number): Promise<Check> {
  return post('/api/checks', { tableNo })
}

export function addOrder(checkId: number, items: OrderLine[]): Promise<Order> {
  return post(`/api/checks/${checkId}/orders`, { items })
}

export function payCheck(id: number): Promise<Check> {
  return post(`/api/checks/${id}/pay`)
}

export function closeCheck(id: number): Promise<Check> {
  return post(`/api/checks/${id}/close`)
}
