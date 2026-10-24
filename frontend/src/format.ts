const yenFormat = new Intl.NumberFormat('ja-JP', { style: 'currency', currency: 'JPY' })

const timeFormat = new Intl.DateTimeFormat('ja-JP', { hour: '2-digit', minute: '2-digit' })

// yen は円単位の整数を "￥1,580" の形にする。
export function yen(amount: number): string {
  return yenFormat.format(amount)
}

// time は API の時刻 (RFC 3339) を "18:05" の形にする。
export function time(iso: string): string {
  return timeFormat.format(new Date(iso))
}
