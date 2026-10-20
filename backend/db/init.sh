#!/bin/sh
# データベースを初めて作るときに、スキーマと開発用データを流し込む。
#
# PostgreSQL の公式イメージは、データ領域が空のときに限り
# /docker-entrypoint-initdb.d 直下の *.sql と *.sh を名前順に実行する。
# 直下しか見ないため、migrations と seeds をこの順で明示的に実行する。
set -eu

dir=$(dirname "$0")

for f in "$dir"/migrations/*.sql "$dir"/seeds/*.sql; do
	echo "init.sh: $f を実行します"
	# ON_ERROR_STOP を付けないと、psql は失敗した文を飛ばして続きを実行してしまう。
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --file "$f"
done
