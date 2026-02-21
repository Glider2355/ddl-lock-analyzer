package meta

import (
	"database/sql"
	"fmt"
	"strings"
)

// Collector はテーブルメタデータを取得するためのインターフェース。
type Collector interface {
	GetTableMeta(schema, table string) (*TableMeta, error)
	GetMySQLVersion() string
}

// DBCollector はMySQL接続からメタデータを取得する。
type DBCollector struct {
	db           *sql.DB
	database     string
	mysqlVersion string
}

// NewDBCollector は新しい DBCollector を作成する。
func NewDBCollector(db *sql.DB, database string) (*DBCollector, error) {
	c := &DBCollector{db: db, database: database}
	var version string
	if err := db.QueryRow("SELECT @@version").Scan(&version); err != nil {
		return nil, fmt.Errorf("failed to get MySQL version: %w", err)
	}
	c.mysqlVersion = version
	return c, nil
}

// GetMySQLVersion はMySQLバージョンを返す。
func (c *DBCollector) GetMySQLVersion() string {
	return c.mysqlVersion
}

// GetTableMeta は指定テーブルのメタデータを取得する。
func (c *DBCollector) GetTableMeta(schema, table string) (*TableMeta, error) {
	if schema == "" {
		schema = c.database
	}

	tm := &TableMeta{
		Schema:       schema,
		Table:        table,
		MySQLVersion: c.mysqlVersion,
	}

	if err := c.fetchTableInfo(tm); err != nil {
		return nil, err
	}
	if err := c.fetchColumns(tm); err != nil {
		return nil, err
	}
	if err := c.fetchIndexes(tm); err != nil {
		return nil, err
	}
	if err := c.fetchForeignKeys(tm); err != nil {
		return nil, err
	}
	if err := c.fetchReferencedBy(tm); err != nil {
		return nil, err
	}

	return tm, nil
}

func (c *DBCollector) fetchTableInfo(tm *TableMeta) error {
	query := `SELECT ENGINE, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`
	var engine sql.NullString
	var rows, dataLen, idxLen sql.NullInt64
	if err := c.db.QueryRow(query, tm.Schema, tm.Table).Scan(&engine, &rows, &dataLen, &idxLen); err != nil {
		return fmt.Errorf("failed to query table info: %w", err)
	}
	tm.Engine = engine.String
	tm.RowCount = rows.Int64
	tm.DataLength = dataLen.Int64
	tm.IndexLength = idxLen.Int64
	return nil
}

func (c *DBCollector) fetchColumns(tm *TableMeta) error {
	query := `SELECT COLUMN_NAME, ORDINAL_POSITION, DATA_TYPE, COLUMN_TYPE,
		IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA,
		CHARACTER_SET_NAME, COLLATION_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`
	rows, err := c.db.Query(query, tm.Schema, tm.Table)
	if err != nil {
		return fmt.Errorf("failed to query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var col ColumnMeta
		var isNullable string
		var defaultVal, charset, collation sql.NullString
		if err := rows.Scan(&col.Name, &col.OrdinalPos, &col.DataType, &col.ColumnType,
			&isNullable, &col.ColumnKey, &defaultVal, &col.Extra,
			&charset, &collation); err != nil {
			return fmt.Errorf("failed to scan column: %w", err)
		}
		col.IsNullable = strings.EqualFold(isNullable, "YES")
		col.DefaultValue = defaultVal.String
		col.CharacterSet = charset.String
		col.Collation = collation.String
		tm.Columns = append(tm.Columns, col)
	}
	return rows.Err()
}

func (c *DBCollector) fetchIndexes(tm *TableMeta) error {
	query := `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`
	rows, err := c.db.Query(query, tm.Schema, tm.Table)
	if err != nil {
		return fmt.Errorf("failed to query indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	indexMap := make(map[string]*IndexMeta)
	var indexOrder []string
	for rows.Next() {
		var indexName, colName, indexType string
		var nonUnique int
		if err := rows.Scan(&indexName, &colName, &nonUnique, &indexType); err != nil {
			return fmt.Errorf("failed to scan index: %w", err)
		}
		idx, ok := indexMap[indexName]
		if !ok {
			idx = &IndexMeta{
				Name:      indexName,
				IsUnique:  nonUnique == 0,
				IsPrimary: indexName == "PRIMARY",
				IndexType: indexType,
			}
			indexMap[indexName] = idx
			indexOrder = append(indexOrder, indexName)
		}
		idx.Columns = append(idx.Columns, colName)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range indexOrder {
		tm.Indexes = append(tm.Indexes, *indexMap[name])
	}
	return nil
}

func (c *DBCollector) fetchForeignKeys(tm *TableMeta) error {
	query := `SELECT kcu.CONSTRAINT_NAME, kcu.TABLE_SCHEMA, kcu.TABLE_NAME,
		kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_SCHEMA, kcu.REFERENCED_TABLE_NAME,
		kcu.REFERENCED_COLUMN_NAME, rc.DELETE_RULE, rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`
	rows, err := c.db.Query(query, tm.Schema, tm.Table)
	if err != nil {
		return fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	fkMap := make(map[string]*ForeignKeyMeta)
	var fkOrder []string
	for rows.Next() {
		var name, srcSchema, srcTable, srcCol, refSchema, refTable, refCol, onDel, onUpd string
		if err := rows.Scan(&name, &srcSchema, &srcTable, &srcCol,
			&refSchema, &refTable, &refCol, &onDel, &onUpd); err != nil {
			return fmt.Errorf("failed to scan FK: %w", err)
		}
		fk, ok := fkMap[name]
		if !ok {
			fk = &ForeignKeyMeta{
				ConstraintName:   name,
				SourceSchema:     srcSchema,
				SourceTable:      srcTable,
				ReferencedSchema: refSchema,
				ReferencedTable:  refTable,
				OnDelete:         onDel,
				OnUpdate:         onUpd,
			}
			fkMap[name] = fk
			fkOrder = append(fkOrder, name)
		}
		fk.SourceColumns = append(fk.SourceColumns, srcCol)
		fk.ReferencedColumns = append(fk.ReferencedColumns, refCol)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range fkOrder {
		tm.ForeignKeys = append(tm.ForeignKeys, *fkMap[name])
	}
	return nil
}

func (c *DBCollector) fetchReferencedBy(tm *TableMeta) error {
	query := `SELECT kcu.CONSTRAINT_NAME, kcu.TABLE_SCHEMA, kcu.TABLE_NAME,
		kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_SCHEMA, kcu.REFERENCED_TABLE_NAME,
		kcu.REFERENCED_COLUMN_NAME, rc.DELETE_RULE, rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.REFERENCED_TABLE_SCHEMA = ? AND kcu.REFERENCED_TABLE_NAME = ?
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`
	rows, err := c.db.Query(query, tm.Schema, tm.Table)
	if err != nil {
		return fmt.Errorf("failed to query referenced_by: %w", err)
	}
	defer func() { _ = rows.Close() }()

	fkMap := make(map[string]*ForeignKeyMeta)
	var fkOrder []string
	for rows.Next() {
		var name, srcSchema, srcTable, srcCol, refSchema, refTable, refCol, onDel, onUpd string
		if err := rows.Scan(&name, &srcSchema, &srcTable, &srcCol,
			&refSchema, &refTable, &refCol, &onDel, &onUpd); err != nil {
			return fmt.Errorf("failed to scan referenced_by FK: %w", err)
		}
		fk, ok := fkMap[name]
		if !ok {
			fk = &ForeignKeyMeta{
				ConstraintName:   name,
				SourceSchema:     srcSchema,
				SourceTable:      srcTable,
				ReferencedSchema: refSchema,
				ReferencedTable:  refTable,
				OnDelete:         onDel,
				OnUpdate:         onUpd,
			}
			fkMap[name] = fk
			fkOrder = append(fkOrder, name)
		}
		fk.SourceColumns = append(fk.SourceColumns, srcCol)
		fk.ReferencedColumns = append(fk.ReferencedColumns, refCol)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range fkOrder {
		tm.ReferencedBy = append(tm.ReferencedBy, *fkMap[name])
	}
	return nil
}
