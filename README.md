# ddl-lock-analyzer

MySQL の `ALTER TABLE` 実行前に、ロック影響を予測する CLI ツール。

本番適用前に DDL の影響範囲を事前評価し、安全なスキーマ変更を支援します。

## 予測する項目

- **ALGORITHM** — INSTANT / INPLACE / COPY
- **LOCK レベル** — NONE / SHARED / EXCLUSIVE
- **テーブル再構築** の有無
- **リスクレベル** — LOW / MEDIUM / HIGH / CRITICAL
- **外部キー依存テーブルへの MDL 伝播**
- **テーブル情報** (行数・データサイズ・インデックス数)

## インストール

```bash
go install github.com/Glider2355/ddl-lock-analyzer@latest
```

または、ソースからビルド:

```bash
git clone https://github.com/Glider2355/ddl-lock-analyzer.git
cd ddl-lock-analyzer
make build
# ./bin/ddl-lock-analyzer が生成されます
```

## 使い方

```bash
# DSN で接続
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)" \
  --dsn "root:pass@tcp(localhost:3306)/mydb"

# 個別フラグで接続
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)" \
  --host localhost --port 3306 --user root --password pass --database mydb

# SQL ファイルから読み込み
ddl-lock-analyzer analyze \
  --file ./migrations/001_add_column.sql \
  --dsn "root:pass@tcp(localhost:3306)/mydb"

# JSON 出力
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users MODIFY COLUMN email VARCHAR(512) NOT NULL" \
  --dsn "root:pass@tcp(localhost:3306)/mydb" \
  --format json
```

## 出力例

### LOW リスク — INSTANT (カラム追加)

```
=== DDL Lock Analysis Report ===

Table: mydb.users
SQL:   ALTER TABLE `users` ADD COLUMN `nickname` VARCHAR(255)

  Operation     : ADD COLUMN (末尾, NULLABLE)
  Algorithm     : INSTANT
  Lock Level    : NONE (concurrent DML allowed)
  Table Rebuild : No
  Table Info    : rows: ~500,000, data: 120MB, indexes: 3
  Risk Level    : LOW

  Note:
    - INSTANT algorithm available (MySQL 8.0.12+)
    - No table rebuild required
    - DML operations are not blocked
```

### CRITICAL リスク — COPY (カラム型変更)

```
=== DDL Lock Analysis Report ===

Table: mydb.users
SQL:   ALTER TABLE `users` MODIFY COLUMN `email` VARCHAR(512) NOT NULL

  Operation     : MODIFY COLUMN (type change)
  Algorithm     : COPY
  Lock Level    : EXCLUSIVE (DML blocked)
  Table Rebuild : Yes
  Table Info    : rows: ~1,200,000, data: 480MB, indexes: 5
  Risk Level    : CRITICAL

  Warning:
    - EXCLUSIVE lock will block all DML during execution
    - Table rebuild required — full table copy
    - Consider using pt-online-schema-change or gh-ost for large tables
```

### JSON 出力

```json
{
  "analyses": [
    {
      "table": "mydb.users",
      "sql": "ALTER TABLE `users` MODIFY COLUMN `email` VARCHAR(512) NOT NULL",
      "operation": "MODIFY_COLUMN",
      "algorithm": "COPY",
      "lock_level": "EXCLUSIVE",
      "table_rebuild": true,
      "table_info": {
        "row_count": 1200000,
        "data_size_bytes": 503316480,
        "index_size_bytes": 52428800,
        "index_count": 5
      },
      "risk_level": "CRITICAL",
      "warnings": [
        "EXCLUSIVE lock will block all DML during execution",
        "Table rebuild required — full table copy",
        "Consider using pt-online-schema-change or gh-ost for large tables"
      ]
    }
  ]
}
```

## リスクレベル

| レベル | 条件 | 説明 |
|--------|------|------|
| **LOW** | INSTANT | メタデータ変更のみ。DML への影響なし |
| **MEDIUM** | INPLACE, Lock=NONE, Rebuild=No | オンライン実行可能だが一定の負荷あり |
| **HIGH** | INPLACE, Rebuild=Yes | テーブル再構築が発生。大テーブルでは長時間 |
| **CRITICAL** | COPY or EXCLUSIVE | DML がブロックされる。サービス影響の可能性大 |

## 対応する ALTER 操作

| カテゴリ | 操作 | 想定 Algorithm |
|----------|------|---------------|
| カラム | ADD COLUMN (末尾/途中) | INSTANT |
| カラム | DROP COLUMN | INSTANT |
| カラム | RENAME COLUMN | INSTANT |
| カラム | SET/DROP DEFAULT | INSTANT |
| カラム | MODIFY (型変更) | COPY |
| カラム | MODIFY (NULL→NOT NULL) | INPLACE |
| インデックス | ADD INDEX / UNIQUE | INPLACE |
| インデックス | DROP INDEX / RENAME INDEX | INPLACE |
| 主キー | ADD PRIMARY KEY | INPLACE (Rebuild) |
| 主キー | DROP PRIMARY KEY | COPY |
| 外部キー | ADD/DROP FOREIGN KEY | INPLACE |
| テーブル | ENGINE 変更 (同一) | INPLACE (Rebuild) |
| テーブル | ENGINE 変更 (異なる) | COPY |
| テーブル | CONVERT CHARACTER SET | COPY |
| テーブル | ROW_FORMAT 変更 | INPLACE (Rebuild) |
| パーティション | ADD/DROP PARTITION | INPLACE |

## フラグ一覧

```
ddl-lock-analyzer analyze [flags]

Flags:
      --sql string        ALTER 文を直接指定
      --file string       SQL ファイルパス
      --dsn string        MySQL DSN (user:pass@tcp(host:port)/dbname)
      --host string       MySQL ホスト (default "localhost")
      --port int          MySQL ポート (default 3306)
      --user string       MySQL ユーザー
      --password string   MySQL パスワード
      --database string   対象データベース名
      --format string     出力フォーマット: text|json (default "text")
      --fk-checks         foreign_key_checks の想定値 (default true)
      --fk-depth int      FK 依存グラフの最大探索深度 (default 5)
```

## 開発

```bash
# テスト
make test

# lint
make lint

# ビルド
make build

# 全部
make all
```

## 対象環境

- MySQL 8.0+
- Go 1.24+

## 注意事項

- 判定ルールは **InnoDB** を前提としています。MyISAM 等は全て COPY/EXCLUSIVE として扱います
- テーブル情報（行数・データサイズ）は `information_schema` から取得した概算値です
- CRITICAL リスクの操作では `pt-online-schema-change` や `gh-ost` の利用を推奨します

## ライセンス

MIT
