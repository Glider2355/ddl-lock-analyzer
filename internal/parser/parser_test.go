package parser

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func TestParseAddColumn(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN nickname VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Table != "users" {
		t.Errorf("expected table 'users', got %q", ops[0].Table)
	}
	if len(ops[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(ops[0].Actions))
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddColumn {
		t.Errorf("expected ADD_COLUMN, got %s", action.Type)
	}
	if action.Detail.ColumnName != "nickname" {
		t.Errorf("expected column 'nickname', got %q", action.Detail.ColumnName)
	}
}

func TestParseAddColumnFirst(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN id INT FIRST")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.Position != "FIRST" {
		t.Errorf("expected position 'FIRST', got %q", action.Detail.Position)
	}
}

func TestParseAddColumnAfter(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD COLUMN email VARCHAR(255) AFTER name")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Detail.Position != "AFTER name" {
		t.Errorf("expected position 'AFTER name', got %q", action.Detail.Position)
	}
}

func TestParseDropColumn(t *testing.T) {
	ops, err := Parse("ALTER TABLE users DROP COLUMN nickname")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropColumn {
		t.Errorf("expected DROP_COLUMN, got %s", action.Type)
	}
	if action.Detail.ColumnName != "nickname" {
		t.Errorf("expected column 'nickname', got %q", action.Detail.ColumnName)
	}
}

func TestParseModifyColumn(t *testing.T) {
	ops, err := Parse("ALTER TABLE users MODIFY COLUMN email VARCHAR(512) NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionModifyColumn {
		t.Errorf("expected MODIFY_COLUMN, got %s", action.Type)
	}
	if action.Detail.IsNullable == nil || *action.Detail.IsNullable {
		t.Error("expected NOT NULL")
	}
}

func TestParseChangeColumn(t *testing.T) {
	ops, err := Parse("ALTER TABLE users CHANGE COLUMN name full_name VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeColumn {
		t.Errorf("expected CHANGE_COLUMN, got %s", action.Type)
	}
	if action.Detail.OldColumnName != "name" {
		t.Errorf("expected old column 'name', got %q", action.Detail.OldColumnName)
	}
	if action.Detail.ColumnName != "full_name" {
		t.Errorf("expected new column 'full_name', got %q", action.Detail.ColumnName)
	}
}

func TestParseRenameColumn(t *testing.T) {
	ops, err := Parse("ALTER TABLE users RENAME COLUMN name TO full_name")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionRenameColumn {
		t.Errorf("expected RENAME_COLUMN, got %s", action.Type)
	}
}

func TestParseAlterColumnSetDefault(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionSetDefault {
		t.Errorf("expected ALTER_COLUMN_SET_DEFAULT, got %s", action.Type)
	}
}

func TestParseAlterColumnDropDefault(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ALTER COLUMN status DROP DEFAULT")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropDefault {
		t.Errorf("expected ALTER_COLUMN_DROP_DEFAULT, got %s", action.Type)
	}
}

func TestParseAddIndex(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD INDEX idx_email (email)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddIndex {
		t.Errorf("expected ADD_INDEX, got %s", action.Type)
	}
	if action.Detail.IndexName != "idx_email" {
		t.Errorf("expected index 'idx_email', got %q", action.Detail.IndexName)
	}
}

func TestParseAddUniqueIndex(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD UNIQUE INDEX idx_email (email)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddUniqueIndex {
		t.Errorf("expected ADD_UNIQUE_INDEX, got %s", action.Type)
	}
}

func TestParseDropIndex(t *testing.T) {
	ops, err := Parse("ALTER TABLE users DROP INDEX idx_email")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropIndex {
		t.Errorf("expected DROP_INDEX, got %s", action.Type)
	}
}

func TestParseRenameIndex(t *testing.T) {
	ops, err := Parse("ALTER TABLE users RENAME INDEX idx_old TO idx_new")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionRenameIndex {
		t.Errorf("expected RENAME_INDEX, got %s", action.Type)
	}
	if action.Detail.OldIndexName != "idx_old" {
		t.Errorf("expected old index 'idx_old', got %q", action.Detail.OldIndexName)
	}
}

func TestParseAddPrimaryKey(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ADD PRIMARY KEY (id)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddPrimaryKey {
		t.Errorf("expected ADD_PRIMARY_KEY, got %s", action.Type)
	}
}

func TestParseDropPrimaryKey(t *testing.T) {
	ops, err := Parse("ALTER TABLE users DROP PRIMARY KEY")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropPrimaryKey {
		t.Errorf("expected DROP_PRIMARY_KEY, got %s", action.Type)
	}
}

func TestParseAddForeignKey(t *testing.T) {
	ops, err := Parse("ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionAddForeignKey {
		t.Errorf("expected ADD_FOREIGN_KEY, got %s", action.Type)
	}
	if action.Detail.RefTable != "users" {
		t.Errorf("expected ref table 'users', got %q", action.Detail.RefTable)
	}
}

func TestParseDropForeignKey(t *testing.T) {
	ops, err := Parse("ALTER TABLE orders DROP FOREIGN KEY fk_user")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionDropForeignKey {
		t.Errorf("expected DROP_FOREIGN_KEY, got %s", action.Type)
	}
}

func TestParseEngineChange(t *testing.T) {
	ops, err := Parse("ALTER TABLE users ENGINE=InnoDB")
	if err != nil {
		t.Fatal(err)
	}
	action := ops[0].Actions[0]
	if action.Type != meta.ActionChangeEngine {
		t.Errorf("expected CHANGE_ENGINE, got %s", action.Type)
	}
	if action.Detail.Engine != "InnoDB" {
		t.Errorf("expected engine 'InnoDB', got %q", action.Detail.Engine)
	}
}

func TestParseMultipleStatements(t *testing.T) {
	sql := `
		ALTER TABLE users ADD COLUMN nickname VARCHAR(255);
		ALTER TABLE orders ADD INDEX idx_user (user_id);
	`
	ops, err := Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}
	if ops[0].Table != "users" {
		t.Errorf("expected first table 'users', got %q", ops[0].Table)
	}
	if ops[1].Table != "orders" {
		t.Errorf("expected second table 'orders', got %q", ops[1].Table)
	}
}

func TestParseMultipleActionsInOneStatement(t *testing.T) {
	sql := "ALTER TABLE users ADD COLUMN nickname VARCHAR(255), ADD INDEX idx_nick (nickname)"
	ops, err := Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if len(ops[0].Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(ops[0].Actions))
	}
}

func TestParseNonAlterStatement(t *testing.T) {
	_, err := Parse("SELECT 1")
	if err == nil {
		t.Fatal("expected error for non-ALTER statement")
	}
}

func TestParseInvalidSQL(t *testing.T) {
	_, err := Parse("THIS IS NOT SQL")
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}
}

func TestParseSchemaQualifiedTable(t *testing.T) {
	ops, err := Parse("ALTER TABLE mydb.users ADD COLUMN nickname VARCHAR(255)")
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Schema != "mydb" {
		t.Errorf("expected schema 'mydb', got %q", ops[0].Schema)
	}
	if ops[0].Table != "users" {
		t.Errorf("expected table 'users', got %q", ops[0].Table)
	}
}
