package meta

// Algorithm はDDL実行アルゴリズムを表す。
type Algorithm string

const (
	AlgorithmInstant Algorithm = "INSTANT"
	AlgorithmInplace Algorithm = "INPLACE"
	AlgorithmCopy    Algorithm = "COPY"
)

// LockLevel はDDL実行中のロックレベルを表す。
type LockLevel string

const (
	LockNone      LockLevel = "NONE"
	LockShared    LockLevel = "SHARED"
	LockExclusive LockLevel = "EXCLUSIVE"
)

// RiskLevel はDDL操作のリスクレベルを表す。
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// TableMeta はMySQLテーブルのメタデータを保持する。
type TableMeta struct {
	Schema       string           `json:"schema"`
	Table        string           `json:"table"`
	Engine       string           `json:"engine"`
	RowCount     int64            `json:"row_count"`
	DataLength   int64            `json:"data_length"`
	IndexLength  int64            `json:"index_length"`
	Columns      []ColumnMeta     `json:"columns"`
	Indexes      []IndexMeta      `json:"indexes"`
	ForeignKeys  []ForeignKeyMeta `json:"foreign_keys"`
	ReferencedBy []ForeignKeyMeta `json:"referenced_by"`
	MySQLVersion string           `json:"mysql_version"`
}

// ColumnMeta はテーブルカラムのメタデータを保持する。
type ColumnMeta struct {
	Name         string `json:"name"`
	OrdinalPos   int    `json:"ordinal_position"`
	DataType     string `json:"data_type"`
	ColumnType   string `json:"column_type"`
	IsNullable   bool   `json:"is_nullable"`
	ColumnKey    string `json:"column_key"`
	DefaultValue string `json:"default_value"`
	Extra        string `json:"extra"`
	CharacterSet string `json:"character_set"`
	Collation    string `json:"collation"`
}

// IndexMeta はインデックスのメタデータを保持する。
type IndexMeta struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique"`
	IsPrimary bool     `json:"is_primary"`
	IndexType string   `json:"index_type"`
}

// ForeignKeyMeta は外部キー制約のメタデータを保持する。
type ForeignKeyMeta struct {
	ConstraintName    string   `json:"constraint_name"`
	SourceSchema      string   `json:"source_schema"`
	SourceTable       string   `json:"source_table"`
	SourceColumns     []string `json:"source_columns"`
	ReferencedSchema  string   `json:"referenced_schema"`
	ReferencedTable   string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
	OnDelete          string   `json:"on_delete"`
	OnUpdate          string   `json:"on_update"`
}

// AlterActionType はALTER TABLE操作の種類を表す。
type AlterActionType string

const (
	ActionAddColumn        AlterActionType = "ADD_COLUMN"
	ActionDropColumn       AlterActionType = "DROP_COLUMN"
	ActionModifyColumn     AlterActionType = "MODIFY_COLUMN"
	ActionChangeColumn     AlterActionType = "CHANGE_COLUMN"
	ActionRenameColumn     AlterActionType = "RENAME_COLUMN"
	ActionSetDefault       AlterActionType = "ALTER_COLUMN_SET_DEFAULT"
	ActionDropDefault      AlterActionType = "ALTER_COLUMN_DROP_DEFAULT"
	ActionAddIndex         AlterActionType = "ADD_INDEX"
	ActionAddUniqueIndex   AlterActionType = "ADD_UNIQUE_INDEX"
	ActionAddFulltextIndex AlterActionType = "ADD_FULLTEXT_INDEX"
	ActionDropIndex        AlterActionType = "DROP_INDEX"
	ActionRenameIndex      AlterActionType = "RENAME_INDEX"
	ActionAddPrimaryKey    AlterActionType = "ADD_PRIMARY_KEY"
	ActionDropPrimaryKey   AlterActionType = "DROP_PRIMARY_KEY"
	ActionAddForeignKey    AlterActionType = "ADD_FOREIGN_KEY"
	ActionDropForeignKey   AlterActionType = "DROP_FOREIGN_KEY"
	ActionRenameTable      AlterActionType = "RENAME_TABLE"
	ActionConvertCharset   AlterActionType = "CONVERT_CHARACTER_SET"
	ActionChangeEngine     AlterActionType = "CHANGE_ENGINE"
	ActionChangeRowFormat  AlterActionType = "CHANGE_ROW_FORMAT"
	ActionAddPartition        AlterActionType = "ADD_PARTITION"
	ActionDropPartition       AlterActionType = "DROP_PARTITION"
	ActionAddSpatialIndex     AlterActionType = "ADD_SPATIAL_INDEX"
	ActionChangeAutoIncrement AlterActionType = "CHANGE_AUTO_INCREMENT"
	ActionChangeKeyBlockSize  AlterActionType = "CHANGE_KEY_BLOCK_SIZE"
	ActionForceRebuild        AlterActionType = "FORCE_REBUILD"
	ActionCoalescePartition   AlterActionType = "COALESCE_PARTITION"
	ActionReorganizePartition AlterActionType = "REORGANIZE_PARTITION"
	ActionTruncatePartition   AlterActionType = "TRUNCATE_PARTITION"
	ActionRebuildPartition    AlterActionType = "REBUILD_PARTITION"
	ActionRemovePartitioning  AlterActionType = "REMOVE_PARTITIONING"
	ActionPartitionBy         AlterActionType = "PARTITION_BY"
	ActionExchangePartition   AlterActionType = "EXCHANGE_PARTITION"
)

// ActionDetail はALTER操作の詳細情報を保持する。
type ActionDetail struct {
	ColumnName     string   `json:"column_name,omitempty"`
	OldColumnName  string   `json:"old_column_name,omitempty"`
	ColumnType     string   `json:"column_type,omitempty"`
	OldColumnType  string   `json:"old_column_type,omitempty"`
	IsNullable     *bool    `json:"is_nullable,omitempty"`
	WasNullable    *bool    `json:"was_nullable,omitempty"`
	DefaultValue   string   `json:"default_value,omitempty"`
	Position       string   `json:"position,omitempty"` // "", "FIRST", "AFTER <col>"
	IndexName      string   `json:"index_name,omitempty"`
	IndexColumns   []string `json:"index_columns,omitempty"`
	OldIndexName   string   `json:"old_index_name,omitempty"`
	ConstraintName string   `json:"constraint_name,omitempty"`
	Engine         string   `json:"engine,omitempty"`
	Charset        string   `json:"charset,omitempty"`
	RowFormat      string   `json:"row_format,omitempty"`
	// カラム属性
	IsAutoIncrement bool   `json:"is_auto_increment,omitempty"`
	GeneratedType   string `json:"generated_type,omitempty"` // "", "STORED", "VIRTUAL"
	// FK詳細
	RefTable   string   `json:"ref_table,omitempty"`
	RefColumns []string `json:"ref_columns,omitempty"`
}

// AlterAction は単一のALTER TABLEアクションを表す。
type AlterAction struct {
	Type   AlterActionType `json:"type"`
	Detail ActionDetail    `json:"detail"`
}

// AlterOperation はパースされたALTER TABLE文を表す。
type AlterOperation struct {
	Table   string        `json:"table"`
	Schema  string        `json:"schema"`
	Actions []AlterAction `json:"actions"`
	RawSQL  string        `json:"raw_sql"`
}
