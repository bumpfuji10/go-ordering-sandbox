// Package env は環境変数の読み取りを扱う。
package env

import "os"

// Or は環境変数を読み、空であれば既定値を返す。
//
// os.Getenv は未設定と空文字を区別しない。どちらも「指定がない」と
// みなして既定値に倒すため、この用途では区別する必要がない。
func Or(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
