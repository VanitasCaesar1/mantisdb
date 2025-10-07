package sql

import (
	"fmt"
	"time"
)

// AST node types for SQL parsing

// Node represents a node in the SQL AST
type Node interface {
	String() string
	Accept(visitor Visitor) interface{}
}

// Statement represents a SQL statement
type Statement interface {
	Node
	StatementNode()
}

// Expression represents a SQL expression
type Expression interface {
	Node
	ExpressionNode()
}

// Visitor interface for AST traversal
type Visitor interface {
	VisitSelectStatement(*SelectStatement) interface{}
	VisitInsertStatement(*InsertStatement) interface{}
	VisitUpdateStatement(*UpdateStatement) interface{}
	VisitDeleteStatement(*DeleteStatement) interface{}
	VisitCreateTableStatement(*CreateTableStatement) interface{}
	VisitDropTableStatement(*DropTableStatement) interface{}
	VisitAlterTableStatement(*AlterTableStatement) interface{}
	VisitCreateIndexStatement(*CreateIndexStatement) interface{}
	VisitDropIndexStatement(*DropIndexStatement) interface{}
	VisitBeginTransactionStatement(*BeginTransactionStatement) interface{}
	VisitCommitTransactionStatement(*CommitTransactionStatement) interface{}
	VisitRollbackTransactionStatement(*RollbackTransactionStatement) interface{}
	VisitSavepointStatement(*SavepointStatement) interface{}
	VisitReleaseSavepointStatement(*ReleaseSavepointStatement) interface{}
	VisitBinaryExpression(*BinaryExpression) interface{}
	VisitUnaryExpression(*UnaryExpression) interface{}
	VisitLiteralExpression(*LiteralExpression) interface{}
	VisitIdentifierExpression(*IdentifierExpression) interface{}
	VisitFunctionCall(*FunctionCall) interface{}
	VisitSubquery(*Subquery) interface{}
	VisitCaseExpression(*CaseExpression) interface{}
}

// Statements

type SelectStatement struct {
	Distinct   bool
	Fields     []SelectField
	From       []TableReference
	Where      Expression
	GroupBy    []Expression
	Having     Expression
	OrderBy    []OrderByClause
	Limit      *LimitClause
	Offset     *OffsetClause
	With       []*CommonTableExpression
	WindowDefs []*WindowDefinition
}

func (s *SelectStatement) StatementNode() {}
func (s *SelectStatement) String() string { return "SELECT" }
func (s *SelectStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitSelectStatement(s)
}

type InsertStatement struct {
	Table      *TableReference
	Columns    []string
	Values     [][]Expression
	Select     *SelectStatement
	OnConflict *OnConflictClause
}

func (s *InsertStatement) StatementNode() {}
func (s *InsertStatement) String() string { return "INSERT" }
func (s *InsertStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitInsertStatement(s)
}

type UpdateStatement struct {
	Table *TableReference
	Set   []SetClause
	From  []TableReference
	Where Expression
	Joins []*JoinClause
}

func (s *UpdateStatement) StatementNode() {}
func (s *UpdateStatement) String() string { return "UPDATE" }
func (s *UpdateStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitUpdateStatement(s)
}

type DeleteStatement struct {
	From  *TableReference
	Using []TableReference
	Where Expression
	Joins []*JoinClause
}

func (s *DeleteStatement) StatementNode() {}
func (s *DeleteStatement) String() string { return "DELETE" }
func (s *DeleteStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitDeleteStatement(s)
}

type CreateTableStatement struct {
	Table       *TableReference
	Columns     []*ColumnDefinition
	Constraints []*TableConstraint
	IfNotExists bool
	Temporary   bool
}

func (s *CreateTableStatement) StatementNode() {}
func (s *CreateTableStatement) String() string { return "CREATE TABLE" }
func (s *CreateTableStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitCreateTableStatement(s)
}

type DropTableStatement struct {
	Tables   []*TableReference
	IfExists bool
	Cascade  bool
}

func (s *DropTableStatement) StatementNode() {}
func (s *DropTableStatement) String() string { return "DROP TABLE" }
func (s *DropTableStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitDropTableStatement(s)
}

type AlterTableStatement struct {
	Table   *TableReference
	Actions []AlterTableAction
}

func (s *AlterTableStatement) StatementNode() {}
func (s *AlterTableStatement) String() string { return "ALTER TABLE" }
func (s *AlterTableStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitAlterTableStatement(s)
}

type CreateIndexStatement struct {
	Name        string
	Table       *TableReference
	Columns     []IndexColumn
	Unique      bool
	IfNotExists bool
	Where       Expression
}

func (s *CreateIndexStatement) StatementNode() {}
func (s *CreateIndexStatement) String() string { return "CREATE INDEX" }
func (s *CreateIndexStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitCreateIndexStatement(s)
}

type DropIndexStatement struct {
	Name     string
	IfExists bool
	Cascade  bool
}

func (s *DropIndexStatement) StatementNode() {}
func (s *DropIndexStatement) String() string { return "DROP INDEX" }
func (s *DropIndexStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitDropIndexStatement(s)
}

// Transaction Statements

type BeginTransactionStatement struct {
	IsolationLevel *SQLIsolationLevel
	ReadOnly       bool
	Deferrable     bool
}

func (s *BeginTransactionStatement) StatementNode() {}
func (s *BeginTransactionStatement) String() string { return "BEGIN TRANSACTION" }
func (s *BeginTransactionStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitBeginTransactionStatement(s)
}

type CommitTransactionStatement struct {
	Chain bool
}

func (s *CommitTransactionStatement) StatementNode() {}
func (s *CommitTransactionStatement) String() string { return "COMMIT TRANSACTION" }
func (s *CommitTransactionStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitCommitTransactionStatement(s)
}

type RollbackTransactionStatement struct {
	Chain     bool
	Savepoint string
}

func (s *RollbackTransactionStatement) StatementNode() {}
func (s *RollbackTransactionStatement) String() string { return "ROLLBACK TRANSACTION" }
func (s *RollbackTransactionStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitRollbackTransactionStatement(s)
}

type SavepointStatement struct {
	Name string
}

func (s *SavepointStatement) StatementNode() {}
func (s *SavepointStatement) String() string { return "SAVEPOINT" }
func (s *SavepointStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitSavepointStatement(s)
}

type ReleaseSavepointStatement struct {
	Name string
}

func (s *ReleaseSavepointStatement) StatementNode() {}
func (s *ReleaseSavepointStatement) String() string { return "RELEASE SAVEPOINT" }
func (s *ReleaseSavepointStatement) Accept(visitor Visitor) interface{} {
	return visitor.VisitReleaseSavepointStatement(s)
}

// Expressions

type BinaryExpression struct {
	Left     Expression
	Operator BinaryOperator
	Right    Expression
}

func (e *BinaryExpression) ExpressionNode() {}
func (e *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left, e.Operator, e.Right)
}
func (e *BinaryExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitBinaryExpression(e)
}

type UnaryExpression struct {
	Operator UnaryOperator
	Operand  Expression
}

func (e *UnaryExpression) ExpressionNode() {}
func (e *UnaryExpression) String() string  { return fmt.Sprintf("(%s %s)", e.Operator, e.Operand) }
func (e *UnaryExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitUnaryExpression(e)
}

type LiteralExpression struct {
	Value interface{}
	Type  LiteralType
}

func (e *LiteralExpression) ExpressionNode() {}
func (e *LiteralExpression) String() string  { return fmt.Sprintf("%v", e.Value) }
func (e *LiteralExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitLiteralExpression(e)
}

type IdentifierExpression struct {
	Name   string
	Schema string
	Table  string
}

func (e *IdentifierExpression) ExpressionNode() {}
func (e *IdentifierExpression) String() string {
	if e.Schema != "" && e.Table != "" {
		return fmt.Sprintf("%s.%s.%s", e.Schema, e.Table, e.Name)
	} else if e.Table != "" {
		return fmt.Sprintf("%s.%s", e.Table, e.Name)
	}
	return e.Name
}
func (e *IdentifierExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitIdentifierExpression(e)
}

type FunctionCall struct {
	Name      string
	Arguments []Expression
	Distinct  bool
	Filter    Expression
	Over      *WindowSpec
}

func (e *FunctionCall) ExpressionNode() {}
func (e *FunctionCall) String() string  { return fmt.Sprintf("%s(...)", e.Name) }
func (e *FunctionCall) Accept(visitor Visitor) interface{} {
	return visitor.VisitFunctionCall(e)
}

type Subquery struct {
	Query *SelectStatement
}

func (e *Subquery) ExpressionNode() {}
func (e *Subquery) String() string  { return "(SELECT ...)" }
func (e *Subquery) Accept(visitor Visitor) interface{} {
	return visitor.VisitSubquery(e)
}

type CaseExpression struct {
	Expression  Expression
	WhenClauses []*WhenClause
	ElseClause  Expression
}

func (e *CaseExpression) ExpressionNode() {}
func (e *CaseExpression) String() string  { return "CASE ... END" }
func (e *CaseExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitCaseExpression(e)
}

// Supporting structures

type SelectField struct {
	Expression Expression
	Alias      string
}

type TableReference struct {
	Name     string
	Schema   string
	Alias    string
	Subquery *SelectStatement
}

type JoinClause struct {
	Type      JoinType
	Table     *TableReference
	Condition Expression
	Using     []string
}

type OrderByClause struct {
	Expression Expression
	Direction  OrderDirection
	NullsFirst bool
}

type LimitClause struct {
	Count Expression
}

type OffsetClause struct {
	Count Expression
}

type CommonTableExpression struct {
	Name    string
	Columns []string
	Query   *SelectStatement
}

type WindowDefinition struct {
	Name string
	Spec *WindowSpec
}

type WindowSpec struct {
	PartitionBy []Expression
	OrderBy     []OrderByClause
	Frame       *WindowFrame
}

type WindowFrame struct {
	Type  FrameType
	Start *FrameBound
	End   *FrameBound
}

type FrameBound struct {
	Type      FrameBoundType
	Preceding bool
	Following bool
	Value     Expression
}

type OnConflictClause struct {
	Columns []string
	Action  ConflictAction
	Set     []SetClause
	Where   Expression
}

type SetClause struct {
	Column string
	Value  Expression
}

type ColumnDefinition struct {
	Name        string
	Type        *DataType
	Constraints []*ColumnConstraint
}

type DataType struct {
	Name      string
	Length    int
	Precision int
	Scale     int
	Array     bool
}

type ColumnConstraint struct {
	Type       ConstraintType
	Name       string
	NotNull    bool
	Unique     bool
	PrimaryKey bool
	References *ForeignKeyReference
	Check      Expression
	Default    Expression
}

type TableConstraint struct {
	Type       ConstraintType
	Name       string
	Columns    []string
	References *ForeignKeyReference
	Check      Expression
}

type ForeignKeyReference struct {
	Table    *TableReference
	Columns  []string
	OnDelete ReferentialAction
	OnUpdate ReferentialAction
}

type AlterTableAction interface {
	AlterTableActionNode()
}

type AddColumnAction struct {
	Column *ColumnDefinition
}

func (a *AddColumnAction) AlterTableActionNode() {}

type DropColumnAction struct {
	Column  string
	Cascade bool
}

func (a *DropColumnAction) AlterTableActionNode() {}

type AlterColumnAction struct {
	Column string
	Action ColumnAlterAction
}

func (a *AlterColumnAction) AlterTableActionNode() {}

type AddConstraintAction struct {
	Constraint *TableConstraint
}

func (a *AddConstraintAction) AlterTableActionNode() {}

type DropConstraintAction struct {
	Name    string
	Cascade bool
}

func (a *DropConstraintAction) AlterTableActionNode() {}

type IndexColumn struct {
	Name       string
	Expression Expression
	Direction  OrderDirection
}

type WhenClause struct {
	Condition Expression
	Result    Expression
}

// Enums

type BinaryOperator int

const (
	OpEqual BinaryOperator = iota
	OpNotEqual
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OpLike
	OpNotLike
	OpILike
	OpNotILike
	OpIn
	OpNotIn
	OpExists
	OpNotExists
	OpAnd
	OpOr
	OpPlus
	OpMinus
	OpMultiply
	OpDivide
	OpModulo
	OpConcat
	OpBitwiseAnd
	OpBitwiseOr
	OpBitwiseXor
	OpLeftShift
	OpRightShift
	OpRegexMatch
	OpRegexNotMatch
	OpJsonExtract
	OpJsonExtractText
)

func (op BinaryOperator) String() string {
	switch op {
	case OpEqual:
		return "="
	case OpNotEqual:
		return "!="
	case OpLess:
		return "<"
	case OpLessEqual:
		return "<="
	case OpGreater:
		return ">"
	case OpGreaterEqual:
		return ">="
	case OpLike:
		return "LIKE"
	case OpNotLike:
		return "NOT LIKE"
	case OpILike:
		return "ILIKE"
	case OpNotILike:
		return "NOT ILIKE"
	case OpIn:
		return "IN"
	case OpNotIn:
		return "NOT IN"
	case OpExists:
		return "EXISTS"
	case OpNotExists:
		return "NOT EXISTS"
	case OpAnd:
		return "AND"
	case OpOr:
		return "OR"
	case OpPlus:
		return "+"
	case OpMinus:
		return "-"
	case OpMultiply:
		return "*"
	case OpDivide:
		return "/"
	case OpModulo:
		return "%"
	case OpConcat:
		return "||"
	case OpBitwiseAnd:
		return "&"
	case OpBitwiseOr:
		return "|"
	case OpBitwiseXor:
		return "^"
	case OpLeftShift:
		return "<<"
	case OpRightShift:
		return ">>"
	case OpRegexMatch:
		return "~"
	case OpRegexNotMatch:
		return "!~"
	case OpJsonExtract:
		return "->"
	case OpJsonExtractText:
		return "->>"
	default:
		return "UNKNOWN"
	}
}

type UnaryOperator int

const (
	UnaryOpNot UnaryOperator = iota
	UnaryOpMinus
	UnaryOpPlus
	UnaryOpBitwiseNot
)

func (op UnaryOperator) String() string {
	switch op {
	case UnaryOpNot:
		return "NOT"
	case UnaryOpMinus:
		return "-"
	case UnaryOpPlus:
		return "+"
	case UnaryOpBitwiseNot:
		return "~"
	default:
		return "UNKNOWN"
	}
}

type LiteralType int

const (
	LiteralString LiteralType = iota
	LiteralInteger
	LiteralFloat
	LiteralBoolean
	LiteralNull
	LiteralDate
	LiteralTime
	LiteralTimestamp
	LiteralInterval
	LiteralArray
	LiteralObject
)

type JoinType int

const (
	InnerJoin JoinType = iota
	LeftJoin
	RightJoin
	FullJoin
	CrossJoin
	NaturalJoin
	LeftOuterJoin
	RightOuterJoin
	FullOuterJoin
)

func (jt JoinType) String() string {
	switch jt {
	case InnerJoin:
		return "INNER JOIN"
	case LeftJoin:
		return "LEFT JOIN"
	case RightJoin:
		return "RIGHT JOIN"
	case FullJoin:
		return "FULL JOIN"
	case CrossJoin:
		return "CROSS JOIN"
	case NaturalJoin:
		return "NATURAL JOIN"
	case LeftOuterJoin:
		return "LEFT OUTER JOIN"
	case RightOuterJoin:
		return "RIGHT OUTER JOIN"
	case FullOuterJoin:
		return "FULL OUTER JOIN"
	default:
		return "UNKNOWN JOIN"
	}
}

type OrderDirection int

const (
	Ascending OrderDirection = iota
	Descending
)

func (od OrderDirection) String() string {
	switch od {
	case Ascending:
		return "ASC"
	case Descending:
		return "DESC"
	default:
		return "ASC"
	}
}

type FrameType int

const (
	RowsFrame FrameType = iota
	RangeFrame
)

type FrameBoundType int

const (
	UnboundedPreceding FrameBoundType = iota
	CurrentRow
	UnboundedFollowing
	ValuePreceding
	ValueFollowing
)

type ConflictAction int

const (
	DoNothing ConflictAction = iota
	DoUpdate
)

type ConstraintType int

const (
	NotNullConstraint ConstraintType = iota
	UniqueConstraint
	PrimaryKeyConstraint
	ForeignKeyConstraint
	CheckConstraint
	DefaultConstraint
)

type ReferentialAction int

const (
	NoAction ReferentialAction = iota
	Restrict
	Cascade
	SetNull
	SetDefault
)

type ColumnAlterAction int

const (
	SetDataType ColumnAlterAction = iota
	SetColumnDefault
	DropColumnDefault
	SetNotNull
	DropNotNull
)

// SQL Transaction Isolation Levels
type SQLIsolationLevel int

const (
	SQLReadUncommitted SQLIsolationLevel = iota
	SQLReadCommitted
	SQLRepeatableRead
	SQLSerializable
)

func (l SQLIsolationLevel) String() string {
	switch l {
	case SQLReadUncommitted:
		return "READ UNCOMMITTED"
	case SQLReadCommitted:
		return "READ COMMITTED"
	case SQLRepeatableRead:
		return "REPEATABLE READ"
	case SQLSerializable:
		return "SERIALIZABLE"
	default:
		return "READ COMMITTED"
	}
}

// Convert SQL isolation level to transaction system isolation level
func (l SQLIsolationLevel) ToTransactionIsolationLevel() int {
	return int(l)
}

// Error types for SQL parsing

type ParseError struct {
	Message  string
	Position int
	Line     int
	Column   int
	Token    string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("SQL parse error at line %d, column %d: %s (near '%s')",
		e.Line, e.Column, e.Message, e.Token)
}

type ValidationError struct {
	Message string
	Node    Node
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("SQL validation error: %s", e.Message)
}

// Utility functions

func NewStringLiteral(value string) *LiteralExpression {
	return &LiteralExpression{
		Value: value,
		Type:  LiteralString,
	}
}

func NewIntegerLiteral(value int64) *LiteralExpression {
	return &LiteralExpression{
		Value: value,
		Type:  LiteralInteger,
	}
}

func NewFloatLiteral(value float64) *LiteralExpression {
	return &LiteralExpression{
		Value: value,
		Type:  LiteralFloat,
	}
}

func NewBooleanLiteral(value bool) *LiteralExpression {
	return &LiteralExpression{
		Value: value,
		Type:  LiteralBoolean,
	}
}

func NewNullLiteral() *LiteralExpression {
	return &LiteralExpression{
		Value: nil,
		Type:  LiteralNull,
	}
}

func NewTimestampLiteral(value time.Time) *LiteralExpression {
	return &LiteralExpression{
		Value: value,
		Type:  LiteralTimestamp,
	}
}

func NewIdentifier(name string) *IdentifierExpression {
	return &IdentifierExpression{Name: name}
}

func NewQualifiedIdentifier(table, name string) *IdentifierExpression {
	return &IdentifierExpression{Table: table, Name: name}
}

func NewFullyQualifiedIdentifier(schema, table, name string) *IdentifierExpression {
	return &IdentifierExpression{Schema: schema, Table: table, Name: name}
}
