package parser

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func TestParseAddColumn(t *testing.T) {
	// カラム追加のパースを検証
	ops, err := Parse("ALTER TABLE users ADD COLUMN nickname VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Fatalf("操作数が1であること: got %d", len(ops))
	}
	if ops[0].Table != "users" {
		t.Errorf("テーブル名が'users'であること: got %q", ops[0].Table)
	}
	if len(ops[0].Actions) != 1 {
		t.Fatalf("アクション数が1であること: got %d", len(ops[0].Actions))
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddColumn {
		t.Errorf("アクションタイプがADD_COLUMNであること: got %s", action.Type)
	}
	if action.Detail.ColumnName != "nickname" {
		t.Errorf("カラム名が'nickname'であること: got %q", action.Detail.ColumnName)
	}
}

func TestParseAddColumnFirst(t *testing.T) {
	// FIRST指定のカラム追加を検証
	ops, err := Parse("ALTER TABLE users ADD COLUMN id INT FIRST")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.Position != "FIRST" {
		t.Errorf("ポジションが'FIRST'であること: got %q", action.Detail.Position)
	}
}

func TestParseAddColumnAfter(t *testing.T) {
	// AFTER指定のカラム追加を検証
	ops, err := Parse("ALTER TABLE users ADD COLUMN email VARCHAR(255) AFTER name")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.Position != "AFTER name" {
		t.Errorf("ポジションが'AFTER name'であること: got %q", action.Detail.Position)
	}
}

func TestParseDropColumn(t *testing.T) {
	// カラム削除のパースを検証
	ops, err := Parse("ALTER TABLE users DROP COLUMN nickname")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropColumn {
		t.Errorf("アクションタイプがDROP_COLUMNであること: got %s", action.Type)
	}
	if action.Detail.ColumnName != "nickname" {
		t.Errorf("カラム名が'nickname'であること: got %q", action.Detail.ColumnName)
	}
}

func TestParseModifyColumn(t *testing.T) {
	// カラム変更(MODIFY)のパースを検証
	ops, err := Parse("ALTER TABLE users MODIFY COLUMN email VARCHAR(512) NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionModifyColumn {
		t.Errorf("アクションタイプがMODIFY_COLUMNであること: got %s", action.Type)
	}
	if action.Detail.IsNullable == nil || *action.Detail.IsNullable {
		t.Error("NOT NULLであること")
	}
}

func TestParseChangeColumn(t *testing.T) {
	// カラム変更(CHANGE)のパースを検証
	ops, err := Parse("ALTER TABLE users CHANGE COLUMN name full_name VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeColumn {
		t.Errorf("アクションタイプがCHANGE_COLUMNであること: got %s", action.Type)
	}
	if action.Detail.OldColumnName != "name" {
		t.Errorf("変更前カラム名が'name'であること: got %q", action.Detail.OldColumnName)
	}
	if action.Detail.ColumnName != "full_name" {
		t.Errorf("変更後カラム名が'full_name'であること: got %q", action.Detail.ColumnName)
	}
}

func TestParseRenameColumn(t *testing.T) {
	// カラムリネームのパースを検証
	ops, err := Parse("ALTER TABLE users RENAME COLUMN name TO full_name")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionRenameColumn {
		t.Errorf("アクションタイプがRENAME_COLUMNであること: got %s", action.Type)
	}
}

func TestParseAlterColumnSetDefault(t *testing.T) {
	// ALTER COLUMN SET DEFAULTのパースを検証
	ops, err := Parse("ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionSetDefault {
		t.Errorf("アクションタイプがALTER_COLUMN_SET_DEFAULTであること: got %s", action.Type)
	}
}

func TestParseAlterColumnDropDefault(t *testing.T) {
	// ALTER COLUMN DROP DEFAULTのパースを検証
	ops, err := Parse("ALTER TABLE users ALTER COLUMN status DROP DEFAULT")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropDefault {
		t.Errorf("アクションタイプがALTER_COLUMN_DROP_DEFAULTであること: got %s", action.Type)
	}
}

func TestParseAddIndex(t *testing.T) {
	// インデックス追加のパースを検証
	ops, err := Parse("ALTER TABLE users ADD INDEX idx_email (email)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddIndex {
		t.Errorf("アクションタイプがADD_INDEXであること: got %s", action.Type)
	}
	if action.Detail.IndexName != "idx_email" {
		t.Errorf("インデックス名が'idx_email'であること: got %q", action.Detail.IndexName)
	}
}

func TestParseAddUniqueIndex(t *testing.T) {
	// ユニークインデックス追加のパースを検証
	ops, err := Parse("ALTER TABLE users ADD UNIQUE INDEX idx_email (email)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddUniqueIndex {
		t.Errorf("アクションタイプがADD_UNIQUE_INDEXであること: got %s", action.Type)
	}
}

func TestParseDropIndex(t *testing.T) {
	// インデックス削除のパースを検証
	ops, err := Parse("ALTER TABLE users DROP INDEX idx_email")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropIndex {
		t.Errorf("アクションタイプがDROP_INDEXであること: got %s", action.Type)
	}
}

func TestParseRenameIndex(t *testing.T) {
	// インデックスリネームのパースを検証
	ops, err := Parse("ALTER TABLE users RENAME INDEX idx_old TO idx_new")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionRenameIndex {
		t.Errorf("アクションタイプがRENAME_INDEXであること: got %s", action.Type)
	}
	if action.Detail.OldIndexName != "idx_old" {
		t.Errorf("変更前インデックス名が'idx_old'であること: got %q", action.Detail.OldIndexName)
	}
}

func TestParseAddPrimaryKey(t *testing.T) {
	// 主キー追加のパースを検証
	ops, err := Parse("ALTER TABLE users ADD PRIMARY KEY (id)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddPrimaryKey {
		t.Errorf("アクションタイプがADD_PRIMARY_KEYであること: got %s", action.Type)
	}
}

func TestParseDropPrimaryKey(t *testing.T) {
	// 主キー削除のパースを検証
	ops, err := Parse("ALTER TABLE users DROP PRIMARY KEY")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropPrimaryKey {
		t.Errorf("アクションタイプがDROP_PRIMARY_KEYであること: got %s", action.Type)
	}
}

func TestParseAddForeignKey(t *testing.T) {
	// 外部キー追加のパースを検証
	ops, err := Parse("ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddForeignKey {
		t.Errorf("アクションタイプがADD_FOREIGN_KEYであること: got %s", action.Type)
	}
	if action.Detail.RefTable != "users" {
		t.Errorf("参照テーブルが'users'であること: got %q", action.Detail.RefTable)
	}
}

func TestParseDropForeignKey(t *testing.T) {
	// 外部キー削除のパースを検証
	ops, err := Parse("ALTER TABLE orders DROP FOREIGN KEY fk_user")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropForeignKey {
		t.Errorf("アクションタイプがDROP_FOREIGN_KEYであること: got %s", action.Type)
	}
}

func TestParseEngineChange(t *testing.T) {
	// エンジン変更のパースを検証
	ops, err := Parse("ALTER TABLE users ENGINE=InnoDB")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeEngine {
		t.Errorf("アクションタイプがCHANGE_ENGINEであること: got %s", action.Type)
	}
	if action.Detail.Engine != "InnoDB" {
		t.Errorf("エンジンが'InnoDB'であること: got %q", action.Detail.Engine)
	}
}

func TestParseMultipleStatements(t *testing.T) {
	// 複数SQL文のパースを検証
	sql := `
		ALTER TABLE users ADD COLUMN nickname VARCHAR(255);
		ALTER TABLE orders ADD INDEX idx_user (user_id);
	`
	ops, err := Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("操作数が2であること: got %d", len(ops))
	}
	if ops[0].Table != "users" {
		t.Errorf("1番目のテーブルが'users'であること: got %q", ops[0].Table)
	}
	if ops[1].Table != "orders" {
		t.Errorf("2番目のテーブルが'orders'であること: got %q", ops[1].Table)
	}
}

func TestParseMultipleActionsInOneStatement(t *testing.T) {
	// 1つのALTER文に複数アクションがある場合を検証
	sql := "ALTER TABLE users ADD COLUMN nickname VARCHAR(255), ADD INDEX idx_nick (nickname)"
	ops, err := Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Fatalf("操作数が1であること: got %d", len(ops))
	}
	if len(ops[0].Actions) != 2 {
		t.Fatalf("アクション数が2であること: got %d", len(ops[0].Actions))
	}
}

func TestParseNonAlterStatement(t *testing.T) {
	// ALTER以外のSQL文はエラーになることを検証
	_, err := Parse("SELECT 1")
	if err == nil {
		t.Fatal("ALTER以外の文はエラーになるべき")
	}
}

func TestParseInvalidSQL(t *testing.T) {
	// 不正なSQLはエラーになることを検証
	_, err := Parse("THIS IS NOT SQL")
	if err == nil {
		t.Fatal("不正なSQLはエラーになるべき")
	}
}

func TestParseSchemaQualifiedTable(t *testing.T) {
	// スキーマ修飾テーブル名のパースを検証
	ops, err := Parse("ALTER TABLE mydb.users ADD COLUMN nickname VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Schema != "mydb" {
		t.Errorf("スキーマが'mydb'であること: got %q", ops[0].Schema)
	}
	if ops[0].Table != "users" {
		t.Errorf("テーブルが'users'であること: got %q", ops[0].Table)
	}
}

func TestParseAddColumnAutoIncrement(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN id BIGINT NOT NULL AUTO_INCREMENT")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if !action.Detail.IsAutoIncrement {
		t.Error("AUTO_INCREMENTであること")
	}
	if action.Detail.IsNullable == nil || *action.Detail.IsNullable {
		t.Error("NOT NULLであること")
	}
}

func TestParseAddColumnGenerated(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN full_name VARCHAR(512) GENERATED ALWAYS AS (CONCAT(first_name, ' ', last_name)) STORED")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.GeneratedType != "STORED" {
		t.Errorf("GeneratedTypeが'STORED'であること: got %q", action.Detail.GeneratedType)
	}
}

func TestParseAddColumnVirtualGenerated(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN full_name VARCHAR(512) GENERATED ALWAYS AS (CONCAT(first_name, ' ', last_name)) VIRTUAL")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.GeneratedType != "VIRTUAL" {
		t.Errorf("GeneratedTypeが'VIRTUAL'であること: got %q", action.Detail.GeneratedType)
	}
}

func TestParseForceRebuild(t *testing.T) {
	ops, err := Parse("ALTER TABLE users FORCE")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionForceRebuild {
		t.Errorf("アクションタイプがFORCE_REBUILDであること: got %s", action.Type)
	}
}

func TestParseAutoIncrementValue(t *testing.T) {
	ops, err := Parse("ALTER TABLE users AUTO_INCREMENT=1000")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeAutoIncrement {
		t.Errorf("アクションタイプがCHANGE_AUTO_INCREMENTであること: got %s", action.Type)
	}
}

func TestParseKeyBlockSize(t *testing.T) {
	ops, err := Parse("ALTER TABLE users KEY_BLOCK_SIZE=8")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeKeyBlockSize {
		t.Errorf("アクションタイプがCHANGE_KEY_BLOCK_SIZEであること: got %s", action.Type)
	}
}

func TestParseRemovePartitioning(t *testing.T) {
	ops, err := Parse("ALTER TABLE users REMOVE PARTITIONING")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionRemovePartitioning {
		t.Errorf("アクションタイプがREMOVE_PARTITIONINGであること: got %s", action.Type)
	}
}

func TestParseTruncatePartition(t *testing.T) {
	ops, err := Parse("ALTER TABLE users TRUNCATE PARTITION p0")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionTruncatePartition {
		t.Errorf("アクションタイプがTRUNCATE_PARTITIONであること: got %s", action.Type)
	}
}

func TestParseCoalescePartitions(t *testing.T) {
	ops, err := Parse("ALTER TABLE users COALESCE PARTITION 2")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionCoalescePartition {
		t.Errorf("アクションタイプがCOALESCE_PARTITIONであること: got %s", action.Type)
	}
}

func TestParseModifyColumnAutoIncrement(t *testing.T) {
	ops, err := Parse("ALTER TABLE users MODIFY COLUMN id BIGINT NOT NULL AUTO_INCREMENT")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if !action.Detail.IsAutoIncrement {
		t.Error("AUTO_INCREMENTであること")
	}
}
