# ddl-lock-analyzer 仕様書

## 1. 概要

MySQL の `ALTER TABLE` 実行前に、以下を静的解析＋メタ情報解析で予測する CLI ツール。

- 想定される ALGORITHM（INPLACE / COPY / INSTANT）
- 想定される LOCK レベル（NONE / SHARED / EXCLUSIVE）
- テーブル再構築（Table Rebuild）の有無
- 外部キー依存テーブルへのロック伝播スコープ
- 推定影響時間

本ツールにより、DBA・開発者が本番適用前に DDL の影響範囲を事前評価できる。

## 2. 対象環境

| 項目 | 値 |
|------|-----|
| 対象 RDBMS | MySQL 8.0+ |
| 実装言語 | Go |
| 動作 OS | Linux / macOS / Windows |

## 3. アーキテクチャ

```
┌─────────────────────────────────────────────────┐
│                  CLI (cobra)                     │
│         コマンド解析・出力フォーマット              │
└────────────────┬────────────────────────────────┘
                 │
     ┌───────────┴───────────┐
     ▼                       ▼
┌──────────┐          ┌──────────────┐
│ SQL      │          │ DB Meta      │
│ Parser   │          │ Collector    │
│          │          │              │
│ ALTER文の│          │ テーブル情報  │
│ 静的解析 │          │ 行数・サイズ  │
└────┬─────┘          └──────┬───────┘
     │                       │
     └───────────┬───────────┘
                 ▼
        ┌────────────────┐
        │  FK Resolver   │
        │                │
        │ 外部キー依存   │
        │ グラフ構築      │
        └───────┬────────┘
                │
                ▼
        ┌────────────────┐
        │  Lock          │
        │  Predictor     │
        │                │
        │ ALGORITHM/LOCK │
        │ 判定エンジン    │
        │ + FK伝播解析   │
        └───────┬────────┘
                │
                ▼
        ┌────────────────┐
        │  Reporter      │
        │                │
        │ 結果出力        │
        │ (text/json)    │
        └────────────────┘
```

## 4. コンポーネント詳細

### 4.1 SQL Parser

ALTER TABLE 文を解析し、操作種別を抽出する。

**入力**: SQL 文字列（単一または複数）

**出力**: 構造化された ALTER 操作リスト

```go
type AlterOperation struct {
    Table       string           // テーブル名
    Schema      string           // スキーマ名
    Operations  []AlterAction    // 操作一覧
    RawSQL      string           // 元の SQL
}

type AlterAction struct {
    Type    AlterActionType  // ADD_COLUMN, DROP_COLUMN, MODIFY_COLUMN, ADD_INDEX, etc.
    Detail  ActionDetail     // 各操作の詳細
}
```

**対応する ALTER 操作一覧**:

| カテゴリ | 操作 |
|----------|------|
| カラム操作 | ADD COLUMN, DROP COLUMN, MODIFY COLUMN, CHANGE COLUMN, RENAME COLUMN, ALTER COLUMN SET DEFAULT, ALTER COLUMN DROP DEFAULT |
| インデックス操作 | ADD INDEX, ADD UNIQUE INDEX, ADD FULLTEXT INDEX, DROP INDEX, RENAME INDEX |
| 主キー操作 | ADD PRIMARY KEY, DROP PRIMARY KEY |
| 外部キー操作 | ADD FOREIGN KEY, DROP FOREIGN KEY |
| テーブル操作 | RENAME TABLE, CONVERT TO CHARACTER SET, ENGINE変更, ROW_FORMAT変更, ADD/DROP PARTITION |
| その他 | ALGORITHM指定, LOCK指定（明示指定時の検証に使用） |

### 4.2 DB Meta Collector

MySQL に接続し、対象テーブルのメタ情報を取得する。

**取得情報**:

```go
type TableMeta struct {
    Schema          string
    Table           string
    Engine          string           // InnoDB, MyISAM, etc.
    RowCount        int64            // 推定行数 (information_schema)
    DataLength      int64            // データサイズ (bytes)
    IndexLength     int64            // インデックスサイズ (bytes)
    Columns         []ColumnMeta     // カラム一覧
    Indexes         []IndexMeta      // インデックス一覧
    ForeignKeys     []ForeignKeyMeta // このテーブルが持つ FK (子→親)
    ReferencedBy    []ForeignKeyMeta // このテーブルを参照する FK (親←子)
    MySQLVersion    string           // MySQL バージョン
}

type ForeignKeyMeta struct {
    ConstraintName    string   // 制約名
    SourceSchema      string   // FK を持つテーブルのスキーマ
    SourceTable       string   // FK を持つテーブル (子テーブル)
    SourceColumns     []string // FK カラム
    ReferencedSchema  string   // 参照先スキーマ
    ReferencedTable   string   // 参照先テーブル (親テーブル)
    ReferencedColumns []string // 参照先カラム
    OnDelete          string   // CASCADE, SET NULL, RESTRICT, etc.
    OnUpdate          string   // CASCADE, SET NULL, RESTRICT, etc.
}
```

**データソース**:

- `information_schema.TABLES` — 行数・サイズ
- `information_schema.COLUMNS` — カラム定義
- `information_schema.STATISTICS` — インデックス定義
- `information_schema.KEY_COLUMN_USAGE` — 外部キー（子→親方向）
- `information_schema.REFERENTIAL_CONSTRAINTS` — FK 制約の詳細（ON DELETE/UPDATE アクション）
- `information_schema.TABLE_CONSTRAINTS` — 制約種別
- `@@version` — MySQL バージョン

**FK 依存の逆方向探索**:

対象テーブルを親として参照している子テーブルを `KEY_COLUMN_USAGE.REFERENCED_TABLE_NAME` で逆引きし、`ReferencedBy` フィールドに格納する。

**オフラインモード**:

DB 接続なしでも動作可能とする。この場合、メタ情報を JSON ファイルから読み込む。推定影響時間は算出不可となり、ALGORITHM/LOCK の判定のみ行う。

### 4.3 FK Resolver（外部キー依存グラフ）

ALTER 対象テーブルに関連する外部キー依存を再帰的に探索し、ロック伝播スコープを特定する。

#### 4.3.1 FK 依存グラフの構造

```go
type FKGraph struct {
    Root       string              // ALTER 対象テーブル (schema.table)
    Parents    []FKRelation        // このテーブルが参照する親テーブル群
    Children   []FKRelation        // このテーブルを参照する子テーブル群
    MaxDepth   int                 // 探索した最大深度
}

type FKRelation struct {
    Table          string          // 関連テーブル名 (schema.table)
    Constraint     ForeignKeyMeta  // FK 制約情報
    Direction      FKDirection     // PARENT or CHILD
    Depth          int             // ALTER 対象テーブルからの距離
    LockImpact     FKLockImpact    // このテーブルに波及するロック影響
}

type FKDirection string
const (
    FKDirectionParent FKDirection = "PARENT"  // ALTER対象 → 参照先 (親)
    FKDirectionChild  FKDirection = "CHILD"   // 参照元 (子) → ALTER対象
)

type FKLockImpact struct {
    MetadataLock   bool      // メタデータロック取得の有無
    LockLevel      LockLevel // 予測されるロックレベル
    Reason         string    // ロック取得の理由
}
```

#### 4.3.2 FK ロック伝播ルール

MySQL では ALTER TABLE 実行時、対象テーブルだけでなく外部キーで関連するテーブルにもメタデータロック (MDL) が伝播する。

| シナリオ | 影響先 | ロック種別 | 説明 |
|----------|--------|-----------|------|
| ALTER TABLE (子テーブル) | 親テーブル | SHARED_READ (MDL) | FK 整合性検証のため親テーブルの MDL を取得 |
| ALTER TABLE (親テーブル) | 子テーブル | SHARED_READ (MDL) | 子テーブルの FK 定義参照のため MDL を取得 |
| ADD FOREIGN KEY | 親テーブル | SHARED_READ (MDL) | 参照先の存在・型一致を検証 |
| DROP COLUMN (FK カラム) | 子/親テーブル | EXCLUSIVE (MDL) | FK 制約の暗黙的な変更が伴う場合 |
| 親テーブルのカラム型変更 | 子テーブル | 検証エラー | FK カラムとの型不一致 → ALTER 失敗の可能性 |
| `foreign_key_checks=OFF` | なし | なし | FK 検証スキップにより伝播なし |

**重要**: MDL（メタデータロック）は InnoDB の行ロック/テーブルロックとは別レイヤーで動作する。MDL は DDL 実行中に関連テーブルの DDL を防ぐことが目的であり、DML は通常ブロックしない。ただし、MDL の取得待ちが長時間化すると後続の DML もキューに入り、結果的にブロックされる。

#### 4.3.3 探索アルゴリズム

```
1. ALTER 対象テーブルの FK 情報を取得
2. 親方向の探索:
   a. ForeignKeys (子→親) を走査
   b. 各親テーブルのメタ情報を取得
   c. 親テーブルがさらに FK を持つ場合、再帰的に探索 (最大深度: 5)
3. 子方向の探索:
   a. ReferencedBy (親←子) を走査
   b. 各子テーブルのメタ情報を取得
   c. 子テーブルがさらに子を持つ場合、再帰的に探索 (最大深度: 5)
4. 循環参照を検出した場合はその時点で探索を打ち切り、警告を出力
5. 各関連テーブルに対し、FK ロック伝播ルールに基づき LockImpact を算出
```

#### 4.3.4 `foreign_key_checks` の考慮

- `foreign_key_checks=OFF` の場合、FK 伝播は発生しない
- CLI フラグ `--fk-checks=false` でこの状態をシミュレート可能
- デフォルトは `foreign_key_checks=ON`（MySQL デフォルト準拠）

### 4.4 Lock Predictor（判定エンジン）

MySQL 公式ドキュメント「Online DDL Operations」に基づくルールテーブルで判定する。

#### 4.4.1 判定ルール構造

```go
type PredictionRule struct {
    ActionType     AlterActionType
    Condition      func(action AlterAction, meta *TableMeta) bool
    Algorithm      Algorithm   // INSTANT, INPLACE, COPY
    Lock           LockLevel   // NONE, SHARED, EXCLUSIVE
    TableRebuild   bool
}

type Algorithm string
const (
    AlgorithmInstant Algorithm = "INSTANT"
    AlgorithmInplace Algorithm = "INPLACE"
    AlgorithmCopy    Algorithm = "COPY"
)

type LockLevel string
const (
    LockNone      LockLevel = "NONE"
    LockShared    LockLevel = "SHARED"
    LockExclusive LockLevel = "EXCLUSIVE"
)
```

#### 4.4.2 主要判定ルール（MySQL 8.0）

| 操作 | Algorithm | Lock | Table Rebuild | 備考 |
|------|-----------|------|---------------|------|
| ADD COLUMN (末尾) | INSTANT | NONE | No | MySQL 8.0.12+ |
| ADD COLUMN (途中) | INSTANT | NONE | No | MySQL 8.0.29+ |
| DROP COLUMN | INSTANT | NONE | No | MySQL 8.0.29+ |
| RENAME COLUMN | INSTANT | NONE | No | |
| ALTER COLUMN SET DEFAULT | INSTANT | NONE | No | メタデータのみ変更 |
| ALTER COLUMN DROP DEFAULT | INSTANT | NONE | No | メタデータのみ変更 |
| MODIFY COLUMN (型変更) | COPY | EXCLUSIVE | Yes | |
| MODIFY COLUMN (NULL→NOT NULL) | INPLACE | NONE | Yes | SQL_MODE に依存 |
| ADD INDEX | INPLACE | NONE | No | |
| DROP INDEX | INPLACE | NONE | No | |
| RENAME INDEX | INPLACE | NONE | No | |
| ADD PRIMARY KEY | INPLACE | NONE | Yes | |
| DROP PRIMARY KEY | COPY | EXCLUSIVE | Yes | |
| ADD FOREIGN KEY | INPLACE | NONE | No | foreign_key_checks=OFF 時 |
| DROP FOREIGN KEY | INPLACE | NONE | No | |
| CHANGE ENGINE (同一) | INPLACE | NONE | Yes | |
| CHANGE ENGINE (異なる) | COPY | EXCLUSIVE | Yes | |
| CONVERT CHARACTER SET | COPY | EXCLUSIVE | Yes | データ型が変わる場合 |
| ADD PARTITION | INPLACE | NONE | No | |
| DROP PARTITION | INPLACE | NONE | No | |
| ROW_FORMAT 変更 | INPLACE | NONE | Yes | |

※ ルールは MySQL バージョンによって異なるため、バージョン別のルールセットを保持する。

#### 4.4.3 判定フロー

```
1. ALTER操作種別を特定
2. MySQLバージョンを確認
3. テーブルエンジンを確認 (InnoDB以外は全てCOPY)
4. ルールテーブルから該当ルールを検索
5. 条件関数を評価 (カラム型、既存インデックスなど)
6. Algorithm / Lock / TableRebuild を決定
7. FK依存グラフから関連テーブルへのロック伝播を解析
8. ユーザー明示指定がある場合は互換性を検証
```

#### 4.4.4 推定影響時間の算出

DB 接続モードの場合、以下のヒューリスティクスで概算する。

```
推定時間 = f(操作種別, テーブルサイズ, 行数)
```

| 操作カテゴリ | 計算式 (目安) |
|-------------|--------------|
| INSTANT | ≈ 0 sec (メタデータ変更のみ) |
| INPLACE (Rebuild なし) | DataLength に比例した概算 |
| INPLACE (Rebuild あり) | (DataLength + IndexLength) に比例した概算 |
| COPY | (DataLength + IndexLength) × 係数 (INPLACE より遅い) |

出力は「秒」単位のレンジ表示とする（例: `~30s - ~120s`）。
あくまで目安であり、実際の実行時間はディスク I/O・CPU・同時接続数に依存する旨を警告として表示する。

### 4.5 Reporter

分析結果を整形して出力する。

**出力フォーマット**: `text`（デフォルト）/ `json`

#### text 出力例

```
=== DDL Lock Analysis Report ===

Table: mydb.users
SQL:   ALTER TABLE users ADD COLUMN nickname VARCHAR(255) DEFAULT NULL

  Operation     : ADD COLUMN (末尾, NULLABLE, with DEFAULT)
  Algorithm     : INSTANT
  Lock Level    : NONE (concurrent DML allowed)
  Table Rebuild : No
  Est. Duration : ~0s (metadata only)
  Risk Level    : LOW

  Note:
    - INSTANT algorithm available (MySQL 8.0.12+)
    - No table rebuild required
    - DML operations are not blocked

---

Table: mydb.users
SQL:   ALTER TABLE users MODIFY COLUMN email VARCHAR(512) NOT NULL

  Operation     : MODIFY COLUMN (type change)
  Algorithm     : COPY
  Lock Level    : EXCLUSIVE (DML blocked)
  Table Rebuild : Yes
  Est. Duration : ~45s - ~180s (rows: ~1,200,000, size: ~480MB)
  Risk Level    : HIGH

  Warning:
    - EXCLUSIVE lock will block all DML during execution
    - Table rebuild required — full table copy
    - Consider using pt-online-schema-change or gh-ost for large tables

---

Table: mydb.orders
SQL:   ALTER TABLE orders ADD COLUMN discount_rate DECIMAL(5,2)

  Operation     : ADD COLUMN (末尾, NULLABLE)
  Algorithm     : INSTANT
  Lock Level    : NONE (concurrent DML allowed)
  Table Rebuild : No
  Est. Duration : ~0s (metadata only)
  Risk Level    : LOW

  FK Lock Propagation:
    orders has 3 FK relationships — MDL will propagate to related tables

    Direction  Table                  Lock Type       Reason
    ────────── ────────────────────── ─────────────── ──────────────────────────────
    PARENT     mydb.users             SHARED_READ     FK: orders.user_id → users.id
    PARENT     mydb.products          SHARED_READ     FK: orders.product_id → products.id
    CHILD      mydb.order_items       SHARED_READ     FK: order_items.order_id → orders.id
      └─CHILD  mydb.item_discounts    SHARED_READ     FK: item_discounts.item_id → order_items.id (depth: 2)

  Warning:
    - MDL propagation to 4 related tables detected
    - Long-running DDL on related tables may cause MDL wait queue buildup
    - If concurrent DDL on related tables is planned, coordinate execution order
```

#### json 出力例

```json
{
  "analyses": [
    {
      "table": "mydb.orders",
      "sql": "ALTER TABLE orders ADD COLUMN discount_rate DECIMAL(5,2)",
      "operation": "ADD_COLUMN",
      "algorithm": "INSTANT",
      "lock_level": "NONE",
      "table_rebuild": false,
      "estimated_duration_sec": {
        "min": 0,
        "max": 0
      },
      "risk_level": "LOW",
      "fk_propagation": {
        "total_affected_tables": 4,
        "relations": [
          {
            "direction": "PARENT",
            "table": "mydb.users",
            "constraint": "fk_orders_user_id",
            "columns": ["user_id"],
            "referenced_columns": ["id"],
            "lock_type": "SHARED_READ",
            "depth": 1
          },
          {
            "direction": "PARENT",
            "table": "mydb.products",
            "constraint": "fk_orders_product_id",
            "columns": ["product_id"],
            "referenced_columns": ["id"],
            "lock_type": "SHARED_READ",
            "depth": 1
          },
          {
            "direction": "CHILD",
            "table": "mydb.order_items",
            "constraint": "fk_order_items_order_id",
            "columns": ["order_id"],
            "referenced_columns": ["id"],
            "lock_type": "SHARED_READ",
            "depth": 1
          },
          {
            "direction": "CHILD",
            "table": "mydb.item_discounts",
            "constraint": "fk_item_discounts_item_id",
            "columns": ["item_id"],
            "referenced_columns": ["id"],
            "lock_type": "SHARED_READ",
            "depth": 2
          }
        ]
      },
      "notes": [
        "INSTANT algorithm available (MySQL 8.0.12+)"
      ],
      "warnings": [
        "MDL propagation to 4 related tables detected"
      ]
    }
  ]
}
```

## 5. CLI インターフェース

### 5.1 コマンド体系

```
ddl-lock-analyzer [command]

Commands:
  analyze    ALTER文を解析してロック予測を行う
  version    バージョン情報を表示

Flags:
  -h, --help    ヘルプを表示
```

### 5.2 analyze コマンド

```
ddl-lock-analyzer analyze [flags]

Flags:
      --sql string         解析対象の ALTER 文 (直接指定)
      --file string        解析対象の SQL ファイルパス
      --dsn string         MySQL 接続 DSN (user:pass@tcp(host:port)/dbname)
      --host string        MySQL ホスト (default "localhost")
      --port int           MySQL ポート (default 3306)
      --user string        MySQL ユーザー
      --password string    MySQL パスワード
      --database string    対象データベース名
      --mysql-version string  MySQL バージョン (オフライン時に指定, default "8.0")
      --format string      出力フォーマット: text|json (default "text")
      --fk-checks          foreign_key_checks の想定値 (default true)
      --fk-depth int       FK 依存グラフの最大探索深度 (default 5)
      --offline            オフラインモード (DB接続なし)
      --meta-file string   メタ情報 JSON ファイルパス (オフライン時)
```

### 5.3 使用例

```bash
# 直接 SQL を指定して解析 (DB接続あり)
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)" \
  --dsn "root:pass@tcp(localhost:3306)/mydb"

# SQL ファイルから読み込み
ddl-lock-analyzer analyze \
  --file ./migrations/001_add_column.sql \
  --host localhost --port 3306 --user root --password pass --database mydb

# オフラインモード (DB接続なし)
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users ADD COLUMN nickname VARCHAR(255)" \
  --offline \
  --mysql-version "8.0.32"

# JSON 出力
ddl-lock-analyzer analyze \
  --sql "ALTER TABLE users MODIFY COLUMN email VARCHAR(512)" \
  --dsn "root:pass@tcp(localhost:3306)/mydb" \
  --format json

# 複数 ALTER 文を含む SQL ファイル
ddl-lock-analyzer analyze \
  --file ./migrations/002_multi_alter.sql \
  --dsn "root:pass@tcp(localhost:3306)/mydb"
```

## 6. リスクレベル定義

分析結果にリスクレベルを付与し、直感的に影響度を把握できるようにする。

| レベル | 条件 | 説明 |
|--------|------|------|
| **LOW** | Algorithm=INSTANT | メタデータ変更のみ。DML への影響なし |
| **MEDIUM** | Algorithm=INPLACE, Lock=NONE, Rebuild=No | オンライン実行可能だが、一定の負荷あり |
| **HIGH** | Algorithm=INPLACE, Rebuild=Yes | テーブル再構築が発生。大テーブルでは長時間 |
| **CRITICAL** | Algorithm=COPY or Lock=EXCLUSIVE | DML がブロックされる。サービス影響の可能性大 |

## 7. プロジェクト構成

```
ddl-lock-analyzer/
├── cmd/
│   └── root.go              # CLI エントリポイント (cobra)
│   └── analyze.go           # analyze サブコマンド
│   └── version.go           # version サブコマンド
├── internal/
│   ├── parser/
│   │   ├── parser.go        # SQL パーサー
│   │   └── parser_test.go
│   ├── meta/
│   │   ├── collector.go     # DB メタ情報取得
│   │   ├── collector_test.go
│   │   └── types.go         # メタ情報の型定義
│   ├── fkresolver/
│   │   ├── resolver.go      # FK 依存グラフ構築
│   │   ├── resolver_test.go
│   │   ├── graph.go         # グラフ構造・探索
│   │   └── propagation.go   # ロック伝播ルール
│   ├── predictor/
│   │   ├── predictor.go     # 判定エンジン
│   │   ├── predictor_test.go
│   │   ├── rules.go         # 判定ルールテーブル
│   │   └── duration.go      # 推定時間算出
│   └── reporter/
│       ├── reporter.go      # 結果出力
│       ├── reporter_test.go
│       ├── text.go          # text フォーマッタ
│       └── json.go          # json フォーマッタ
├── docs/
│   └── SPEC.md              # 本仕様書
├── go.mod
├── go.sum
└── main.go
```

## 8. 依存ライブラリ

| ライブラリ | 用途 |
|-----------|------|
| `github.com/spf13/cobra` | CLI フレームワーク |
| `github.com/go-sql-driver/mysql` | MySQL ドライバ |
| `github.com/pingcap/tidb/pkg/parser` | MySQL SQL パーサー |

## 9. 制約・注意事項

1. **InnoDB 前提**: 判定ルールは InnoDB を前提とする。MyISAM 等は全て COPY/EXCLUSIVE として扱う
2. **バージョン依存**: MySQL バージョンにより INSTANT/INPLACE の可否が異なる。ルールは 8.0 系を主対象とし、段階的に 8.4 / 9.x に対応する
3. **推定時間は目安**: 実行時間はハードウェア・負荷状況に大きく依存する。本ツールの推定は参考値である
4. **パーティションテーブル**: パーティション操作の一部は未対応の場合がある
5. **外部ツール推奨**: CRITICAL リスクの操作では `pt-online-schema-change` や `gh-ost` の利用を推奨メッセージとして出力する
6. **FK 伝播解析の前提**: FK ロック伝播は MySQL の MDL (Metadata Lock) の挙動に基づく。MDL の待機はパフォーマンスモニタ (`performance_schema.metadata_locks`) で確認可能だが、本ツールでは静的解析のみ行い、実行時の MDL 競合状態までは検出しない
7. **FK 循環参照**: テーブル間の循環 FK 参照が存在する場合、探索を打ち切り警告を出力する。循環参照自体は MySQL で許容されるが、DDL 実行時にデッドロックリスクがある
8. **オフラインモードでの FK 解析**: オフラインモード時は `--meta-file` に FK 関連情報が含まれていれば伝播解析を行う。含まれていない場合は FK 解析をスキップする
