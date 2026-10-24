import { useState } from 'react'
import { CheckDetail } from './CheckDetail.tsx'
import { CheckList } from './CheckList.tsx'

// App は表示する画面を切り替える。
//
// 画面はテーブル一覧と伝票の 2 つだけのため、ルーティングのライブラリは使わず、
// 開いている伝票の識別子を state に持つ。URL は変わらないため、
// 再読み込みするとテーブル一覧に戻る。
function App() {
  const [checkId, setCheckId] = useState<number | null>(null)

  if (checkId === null) {
    return <CheckList onSelect={setCheckId} />
  }
  return <CheckDetail checkId={checkId} onBack={() => setCheckId(null)} />
}

export default App
