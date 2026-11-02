import type { CheckStatus } from './api.ts'

const labels: Record<CheckStatus, string> = { open: '利用中', paid: '会計済み', closed: '退店済み' }

export function StatusBadge({ status }: { status: CheckStatus }) {
  return <span className={`badge badge-${status}`}>{labels[status]}</span>
}
