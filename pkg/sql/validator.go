package sql

import (
	"fmt"
	"strings"
)

// Validator validates SQL AST nodes for semantic correctness
type Validator struct {
	errors   []ValidationError
	warnings []ValidationWarning
	context  *ValidationContext
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Message string
	Node    Node
}

func (w *ValidationWarning) Error() string {
	return fmt.Sprintf("SQL validation warning: %s", w.Message)
}

// ValidationContext holds context information during validation
type ValidationContext struct {
	tables      map[string]*TableInfo
	columns     map[string]*ValidatorColumnInfo
	aliases     map[string]string
	functions   map[string]*FunctionInfo
	inAggregate bool
	inWindow    bool
	currentCTE  string
}

// TableInfo holds information about a table
type TableInfo struct {
	Name    string
	Schema  string
	Alias   string
	Columns map[string]*ValidatorColumnInfo
}

// ValidatorColumnInfo holds information about a column for validation
type ValidatorColumnInfo struct {
	Name     string
	Table    string
	DataType string
	Nullable bool
}

// FunctionInfo holds information about a function
type FunctionInfo struct {
	Name        string
	MinArgs     int
	MaxArgs     int
	IsAggregate bool
	IsWindow    bool
	ReturnType  string
}

// NewValidator creates a new SQL validator
func NewValidator() *Validator {
	return &Validator{
		context: &ValidationContext{
			tables:    make(map[string]*TableInfo),
			columns:   make(map[string]*ValidatorColumnInfo),
			aliases:   make(map[string]string),
			functions: getBuiltinFunctions(),
		},
	}
}

// Validate validates a SQL statement
func (v *Validator) Validate(stmt Statement) error {
	v.errors = nil
	v.warnings = nil

	stmt.Accept(v)

	if len(v.errors) > 0 {
		return &ValidationError{
			Message: fmt.Sprintf("validation failed with %d errors", len(v.errors)),
		}
	}

	return nil
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() []ValidationError {
	return v.errors
}

// GetWarnings returns all validation warnings
func (v *Validator) GetWarnings() []ValidationWarning {
	return v.warnings
}

// addError adds a validation error
func (v *Validator) addError(message string, node Node) {
	v.errors = append(v.errors, ValidationError{
		Message: message,
		Node:    node,
	})
}

// addErrorSimple adds a validation error without requiring Node interface
func (v *Validator) addErrorSimple(message string) {
	v.errors = append(v.errors, ValidationError{
		Message: message,
		Node:    nil,
	})
}

// addWarning adds a validation warning
func (v *Validator) addWarning(message string, node Node) {
	v.warnings = append(v.warnings, ValidationWarning{
		Message: message,
		Node:    node,
	})
}

// addWarningSimple adds a validation warning without requiring Node interface
func (v *Validator) addWarningSimple(message string) {
	v.warnings = append(v.warnings, ValidationWarning{
		Message: message,
		Node:    nil,
	})
}

// Visitor implementation

func (v *Validator) VisitSelectStatement(stmt *SelectStatement) interface{} {
	// Validate WITH clause (CTEs)
	if stmt.With != nil {
		for _, cte := range stmt.With {
			v.validateCTE(cte)
		}
	}

	// Validate FROM clause first to establish table context
	if stmt.From != nil {
		for _, table := range stmt.From {
			v.validateTableReference(&table)
		}
	}

	// Validate SELECT fields
	for _, field := range stmt.Fields {
		v.validateSelectField(&field)
	}

	// Validate WHERE clause
	if stmt.Where != nil {
		stmt.Where.Accept(v)
	}

	// Validate GROUP BY clause
	if stmt.GroupBy != nil {
		for _, expr := range stmt.GroupBy {
			expr.Accept(v)
		}

		// Check that non-aggregate expressions in SELECT are in GROUP BY
		v.validateGroupByConsistency(stmt)
	}

	// Validate HAVING clause
	if stmt.Having != nil {
		v.context.inAggregate = true
		stmt.Having.Accept(v)
		v.context.inAggregate = false
	}

	// Validate ORDER BY clause
	if stmt.OrderBy != nil {
		for _, clause := range stmt.OrderBy {
			clause.Expression.Accept(v)
		}
	}

	// Validate LIMIT and OFFSET
	if stmt.Limit != nil {
		stmt.Limit.Count.Accept(v)
		v.validateLimitExpression(stmt.Limit.Count)
	}

	if stmt.Offset != nil {
		stmt.Offset.Count.Accept(v)
		v.validateLimitExpression(stmt.Offset.Count)
	}

	// Validate window definitions
	if stmt.WindowDefs != nil {
		for _, windowDef := range stmt.WindowDefs {
			v.validateWindowDefinition(windowDef)
		}
	}

	return nil
}

func (v *Validator) VisitInsertStatement(stmt *InsertStatement) interface{} {
	// Validate table reference
	if stmt.Table != nil {
		v.validateTableReference(stmt.Table)
	}

	// Validate columns and values consistency
	if stmt.Columns != nil && stmt.Values != nil {
		for _, valueRow := range stmt.Values {
			if len(valueRow) != len(stmt.Columns) {
				v.addError("number of values does not match number of columns", stmt)
			}
		}
	}

	// Validate SELECT statement if present
	if stmt.Select != nil {
		stmt.Select.Accept(v)
	}

	// Validate ON CONFLICT clause
	if stmt.OnConflict != nil {
		v.validateOnConflictClause(stmt.OnConflict)
	}

	return nil
}

func (v *Validator) VisitUpdateStatement(stmt *UpdateStatement) interface{} {
	// Validate table reference
	if stmt.Table != nil {
		v.validateTableReference(stmt.Table)
	}

	// Validate SET clauses
	for _, setClause := range stmt.Set {
		v.validateSetClause(&setClause)
	}

	// Validate FROM clause
	if stmt.From != nil {
		for _, table := range stmt.From {
			v.validateTableReference(&table)
		}
	}

	// Validate WHERE clause
	if stmt.Where != nil {
		stmt.Where.Accept(v)
	}

	// Validate JOINs
	if stmt.Joins != nil {
		for _, join := range stmt.Joins {
			v.validateJoinClause(join)
		}
	}

	return nil
}

func (v *Validator) VisitDeleteStatement(stmt *DeleteStatement) interface{} {
	// Validate table reference
	if stmt.From != nil {
		v.validateTableReference(stmt.From)
	}

	// Validate USING clause
	if stmt.Using != nil {
		for _, table := range stmt.Using {
			v.validateTableReference(&table)
		}
	}

	// Validate WHERE clause
	if stmt.Where != nil {
		stmt.Where.Accept(v)
	}

	// Validate JOINs
	if stmt.Joins != nil {
		for _, join := range stmt.Joins {
			v.validateJoinClause(join)
		}
	}

	return nil
}

func (v *Validator) VisitCreateTableStatement(stmt *CreateTableStatement) interface{} {
	// Validate table name
	if stmt.Table == nil || stmt.Table.Name == "" {
		v.addError("table name cannot be empty", stmt)
	}

	// Validate columns
	columnNames := make(map[string]bool)
	for _, col := range stmt.Columns {
		if col.Name == "" {
			v.addError("column name cannot be empty", stmt)
			continue
		}

		if columnNames[col.Name] {
			v.addError(fmt.Sprintf("duplicate column name: %s", col.Name), stmt)
		}
		columnNames[col.Name] = true

		v.validateColumnDefinition(col)
	}

	// Validate table constraints
	for _, constraint := range stmt.Constraints {
		v.validateTableConstraint(constraint, columnNames)
	}

	return nil
}

func (v *Validator) VisitDropTableStatement(stmt *DropTableStatement) interface{} {
	// Validate table names
	for _, table := range stmt.Tables {
		if table.Name == "" {
			v.addError("table name cannot be empty", stmt)
		}
	}

	return nil
}

func (v *Validator) VisitAlterTableStatement(stmt *AlterTableStatement) interface{} {
	// Validate table reference
	if stmt.Table == nil || stmt.Table.Name == "" {
		v.addError("table name cannot be empty", stmt)
	}

	// Validate alter actions
	for _, action := range stmt.Actions {
		v.validateAlterTableAction(action)
	}

	return nil
}

func (v *Validator) VisitCreateIndexStatement(stmt *CreateIndexStatement) interface{} {
	// Validate index name
	if stmt.Name == "" {
		v.addError("index name cannot be empty", stmt)
	}

	// Validate table reference
	if stmt.Table == nil || stmt.Table.Name == "" {
		v.addError("table name cannot be empty", stmt)
	}

	// Validate columns
	if len(stmt.Columns) == 0 {
		v.addError("index must have at least one column", stmt)
	}

	for _, col := range stmt.Columns {
		if col.Name == "" && col.Expression == nil {
			v.addError("index column must have name or expression", stmt)
		}
	}

	// Validate WHERE clause
	if stmt.Where != nil {
		stmt.Where.Accept(v)
	}

	return nil
}

func (v *Validator) VisitDropIndexStatement(stmt *DropIndexStatement) interface{} {
	// Validate index name
	if stmt.Name == "" {
		v.addError("index name cannot be empty", stmt)
	}

	return nil
}

func (v *Validator) VisitBinaryExpression(expr *BinaryExpression) interface{} {
	// Validate operands
	if expr.Left != nil {
		expr.Left.Accept(v)
	}
	if expr.Right != nil {
		expr.Right.Accept(v)
	}

	// Validate operator compatibility
	v.validateBinaryOperator(expr)

	return nil
}

func (v *Validator) VisitUnaryExpression(expr *UnaryExpression) interface{} {
	// Validate operand
	if expr.Operand != nil {
		expr.Operand.Accept(v)
	}

	// Validate operator compatibility
	v.validateUnaryOperator(expr)

	return nil
}

func (v *Validator) VisitLiteralExpression(expr *LiteralExpression) interface{} {
	// Validate literal value
	v.validateLiteralValue(expr)

	return nil
}

func (v *Validator) VisitIdentifierExpression(expr *IdentifierExpression) interface{} {
	// Validate identifier exists in context
	v.validateIdentifier(expr)

	return nil
}

func (v *Validator) VisitFunctionCall(expr *FunctionCall) interface{} {
	// Validate function exists
	funcInfo, exists := v.context.functions[strings.ToUpper(expr.Name)]
	if !exists {
		v.addError(fmt.Sprintf("unknown function: %s", expr.Name), expr)
		return nil
	}

	// Validate argument count
	argCount := len(expr.Arguments)
	if argCount < funcInfo.MinArgs {
		v.addError(fmt.Sprintf("function %s requires at least %d arguments, got %d",
			expr.Name, funcInfo.MinArgs, argCount), expr)
	}
	if funcInfo.MaxArgs >= 0 && argCount > funcInfo.MaxArgs {
		v.addError(fmt.Sprintf("function %s accepts at most %d arguments, got %d",
			expr.Name, funcInfo.MaxArgs, argCount), expr)
	}

	// Validate arguments
	for _, arg := range expr.Arguments {
		arg.Accept(v)
	}

	// Validate aggregate function usage
	if funcInfo.IsAggregate {
		if !v.context.inAggregate && len(v.context.tables) > 0 {
			// Check if we're in a context where aggregates are allowed
			v.addWarning(fmt.Sprintf("aggregate function %s used without GROUP BY", expr.Name), expr)
		}
	}

	// Validate window function usage
	if funcInfo.IsWindow && expr.Over == nil {
		v.addError(fmt.Sprintf("window function %s requires OVER clause", expr.Name), expr)
	}

	// Validate FILTER clause
	if expr.Filter != nil {
		if !funcInfo.IsAggregate {
			v.addError("FILTER clause can only be used with aggregate functions", expr)
		}
		expr.Filter.Accept(v)
	}

	// Validate OVER clause
	if expr.Over != nil {
		v.validateWindowSpec(expr.Over)
	}

	return nil
}

func (v *Validator) VisitSubquery(expr *Subquery) interface{} {
	// Validate subquery
	if expr.Query != nil {
		expr.Query.Accept(v)
	}

	return nil
}

func (v *Validator) VisitCaseExpression(expr *CaseExpression) interface{} {
	// Validate case expression
	if expr.Expression != nil {
		expr.Expression.Accept(v)
	}

	// Validate WHEN clauses
	for _, whenClause := range expr.WhenClauses {
		if whenClause.Condition != nil {
			whenClause.Condition.Accept(v)
		}
		if whenClause.Result != nil {
			whenClause.Result.Accept(v)
		}
	}

	// Validate ELSE clause
	if expr.ElseClause != nil {
		expr.ElseClause.Accept(v)
	}

	// Validate that all result types are compatible
	v.validateCaseResultTypes(expr)

	return nil
}

// Helper validation methods

func (v *Validator) validateCTE(cte *CommonTableExpression) {
	if cte.Name == "" {
		v.addError("CTE name cannot be empty", cte.Query)
	}

	if cte.Query != nil {
		// Save current context
		oldContext := v.context
		v.context = &ValidationContext{
			tables:     make(map[string]*TableInfo),
			columns:    make(map[string]*ValidatorColumnInfo),
			aliases:    make(map[string]string),
			functions:  v.context.functions,
			currentCTE: cte.Name,
		}

		cte.Query.Accept(v)

		// Restore context
		v.context = oldContext

		// Add CTE to current context
		v.context.tables[cte.Name] = &TableInfo{
			Name:    cte.Name,
			Alias:   cte.Name,
			Columns: make(map[string]*ValidatorColumnInfo),
		}
	}
}

func (v *Validator) validateTableReference(table *TableReference) {
	if table.Name == "" && table.Subquery == nil {
		v.addErrorSimple("table reference must have name or subquery")
		return
	}

	if table.Subquery != nil {
		table.Subquery.Accept(v)
	}

	// Add table to context
	tableName := table.Name
	if table.Alias != "" {
		tableName = table.Alias
	}

	v.context.tables[tableName] = &TableInfo{
		Name:    table.Name,
		Schema:  table.Schema,
		Alias:   table.Alias,
		Columns: make(map[string]*ValidatorColumnInfo),
	}
}

func (v *Validator) validateSelectField(field *SelectField) {
	if field.Expression != nil {
		field.Expression.Accept(v)
	}

	// Validate alias uniqueness
	if field.Alias != "" {
		if _, exists := v.context.aliases[field.Alias]; exists {
			v.addError(fmt.Sprintf("duplicate alias: %s", field.Alias), field.Expression)
		}
		v.context.aliases[field.Alias] = field.Alias
	}
}

func (v *Validator) validateGroupByConsistency(stmt *SelectStatement) {
	// This is a simplified check - a full implementation would be more complex
	for _, field := range stmt.Fields {
		if field.Expression != nil {
			v.validateGroupByExpression(field.Expression, stmt.GroupBy)
		}
	}
}

func (v *Validator) validateGroupByExpression(expr Expression, groupBy []Expression) {
	// Check if expression is in GROUP BY or is an aggregate
	// This is a simplified implementation
	switch e := expr.(type) {
	case *FunctionCall:
		if funcInfo, exists := v.context.functions[strings.ToUpper(e.Name)]; exists && funcInfo.IsAggregate {
			return // Aggregate functions are allowed
		}
	case *IdentifierExpression:
		// Check if identifier is in GROUP BY
		for _, groupExpr := range groupBy {
			if v.expressionsEqual(expr, groupExpr) {
				return
			}
		}
		v.addError(fmt.Sprintf("column %s must appear in GROUP BY clause", e.Name), expr)
	}
}

func (v *Validator) validateLimitExpression(expr Expression) {
	// LIMIT/OFFSET must be non-negative integers
	if lit, ok := expr.(*LiteralExpression); ok {
		if lit.Type == LiteralInteger {
			if value, ok := lit.Value.(int64); ok && value < 0 {
				v.addError("LIMIT/OFFSET cannot be negative", expr)
			}
		}
	}
}

func (v *Validator) validateWindowDefinition(windowDef *WindowDefinition) {
	if windowDef.Name == "" {
		v.addErrorSimple("window definition name cannot be empty")
	}

	if windowDef.Spec != nil {
		v.validateWindowSpec(windowDef.Spec)
	}
}

func (v *Validator) validateWindowSpec(spec *WindowSpec) {
	// Validate PARTITION BY expressions
	for _, expr := range spec.PartitionBy {
		expr.Accept(v)
	}

	// Validate ORDER BY expressions
	for _, clause := range spec.OrderBy {
		clause.Expression.Accept(v)
	}

	// Validate frame specification
	if spec.Frame != nil {
		v.validateWindowFrame(spec.Frame)
	}
}

func (v *Validator) validateWindowFrame(frame *WindowFrame) {
	if frame.Start != nil {
		v.validateFrameBound(frame.Start)
	}
	if frame.End != nil {
		v.validateFrameBound(frame.End)
	}

	// Validate frame bounds make sense
	if frame.Start != nil && frame.End != nil {
		v.validateFrameBoundOrder(frame.Start, frame.End)
	}
}

func (v *Validator) validateFrameBound(bound *FrameBound) {
	if bound.Value != nil {
		bound.Value.Accept(v)

		// Value must be non-negative integer
		if lit, ok := bound.Value.(*LiteralExpression); ok {
			if lit.Type == LiteralInteger {
				if value, ok := lit.Value.(int64); ok && value < 0 {
					v.addError("frame bound value cannot be negative", bound.Value)
				}
			}
		}
	}
}

func (v *Validator) validateFrameBoundOrder(start, end *FrameBound) {
	// Validate that start bound comes before end bound
	// This is a simplified check
	if start.Type == UnboundedFollowing {
		v.addErrorSimple("frame start cannot be UNBOUNDED FOLLOWING")
	}
	if end.Type == UnboundedPreceding {
		v.addErrorSimple("frame end cannot be UNBOUNDED PRECEDING")
	}
}

func (v *Validator) validateOnConflictClause(clause *OnConflictClause) {
	// Validate conflict columns exist
	for _, col := range clause.Columns {
		if col == "" {
			v.addErrorSimple("conflict column name cannot be empty")
		}
	}

	// Validate SET clauses if present
	for _, setClause := range clause.Set {
		v.validateSetClause(&setClause)
	}

	// Validate WHERE clause if present
	if clause.Where != nil {
		clause.Where.Accept(v)
	}
}

func (v *Validator) validateSetClause(clause *SetClause) {
	if clause.Column == "" {
		v.addError("SET clause column name cannot be empty", clause.Value)
	}

	if clause.Value != nil {
		clause.Value.Accept(v)
	}
}

func (v *Validator) validateJoinClause(join *JoinClause) {
	if join.Table != nil {
		v.validateTableReference(join.Table)
	}

	if join.Condition != nil {
		join.Condition.Accept(v)
	}

	// Validate USING columns exist
	for _, col := range join.Using {
		if col == "" {
			v.addErrorSimple("USING column name cannot be empty")
		}
	}
}

func (v *Validator) validateColumnDefinition(col *ColumnDefinition) {
	if col.Type == nil {
		v.addErrorSimple(fmt.Sprintf("column %s must have a data type", col.Name))
		return
	}

	// Validate data type
	v.validateDataType(col.Type)

	// Validate constraints
	for _, constraint := range col.Constraints {
		v.validateColumnConstraint(constraint)
	}
}

func (v *Validator) validateDataType(dataType *DataType) {
	if dataType.Name == "" {
		v.addErrorSimple("data type name cannot be empty")
	}

	// Validate type-specific constraints
	switch strings.ToUpper(dataType.Name) {
	case "VARCHAR", "CHAR":
		if dataType.Length <= 0 {
			v.addErrorSimple(fmt.Sprintf("%s type must have positive length", dataType.Name))
		}
	case "DECIMAL", "NUMERIC":
		if dataType.Precision <= 0 {
			v.addErrorSimple(fmt.Sprintf("%s type must have positive precision", dataType.Name))
		}
		if dataType.Scale < 0 || dataType.Scale > dataType.Precision {
			v.addErrorSimple(fmt.Sprintf("%s scale must be between 0 and precision", dataType.Name))
		}
	}
}

func (v *Validator) validateColumnConstraint(constraint *ColumnConstraint) {
	// Validate constraint-specific rules
	switch constraint.Type {
	case ForeignKeyConstraint:
		if constraint.References == nil {
			v.addErrorSimple("foreign key constraint must have references")
		} else {
			v.validateForeignKeyReference(constraint.References)
		}
	case CheckConstraint:
		if constraint.Check == nil {
			v.addErrorSimple("check constraint must have check expression")
		} else {
			constraint.Check.Accept(v)
		}
	case DefaultConstraint:
		if constraint.Default == nil {
			v.addErrorSimple("default constraint must have default expression")
		} else {
			constraint.Default.Accept(v)
		}
	}
}

func (v *Validator) validateTableConstraint(constraint *TableConstraint, columnNames map[string]bool) {
	// Validate constraint columns exist
	for _, col := range constraint.Columns {
		if !columnNames[col] {
			v.addErrorSimple(fmt.Sprintf("constraint references unknown column: %s", col))
		}
	}

	// Validate constraint-specific rules
	switch constraint.Type {
	case ForeignKeyConstraint:
		if constraint.References == nil {
			v.addErrorSimple("foreign key constraint must have references")
		} else {
			v.validateForeignKeyReference(constraint.References)
		}
	case CheckConstraint:
		if constraint.Check == nil {
			v.addErrorSimple("check constraint must have check expression")
		} else {
			constraint.Check.Accept(v)
		}
	}
}

func (v *Validator) validateForeignKeyReference(ref *ForeignKeyReference) {
	if ref.Table == nil || ref.Table.Name == "" {
		v.addErrorSimple("foreign key must reference a table")
	}

	if len(ref.Columns) == 0 {
		v.addErrorSimple("foreign key must reference at least one column")
	}
}

func (v *Validator) validateAlterTableAction(action AlterTableAction) {
	switch a := action.(type) {
	case *AddColumnAction:
		if a.Column != nil {
			v.validateColumnDefinition(a.Column)
		}
	case *DropColumnAction:
		if a.Column == "" {
			v.addErrorSimple("drop column action must specify column name")
		}
	case *AlterColumnAction:
		if a.Column == "" {
			v.addErrorSimple("alter column action must specify column name")
		}
	case *AddConstraintAction:
		if a.Constraint != nil {
			v.validateTableConstraint(a.Constraint, nil) // Column validation would need table info
		}
	case *DropConstraintAction:
		if a.Name == "" {
			v.addErrorSimple("drop constraint action must specify constraint name")
		}
	}
}

func (v *Validator) validateBinaryOperator(expr *BinaryExpression) {
	// Validate operator-specific rules
	switch expr.Operator {
	case OpIn, OpNotIn:
		// Right side should be a list or subquery
		if _, ok := expr.Right.(*Subquery); !ok {
			// In a real implementation, you'd also check for array literals
			v.addWarning("IN operator typically used with subquery or array", expr)
		}
	case OpExists, OpNotExists:
		// Right side should be a subquery
		if _, ok := expr.Right.(*Subquery); !ok {
			v.addError("EXISTS operator requires subquery", expr)
		}
	}
}

func (v *Validator) validateUnaryOperator(expr *UnaryExpression) {
	// Validate unary operator usage
	switch expr.Operator {
	case UnaryOpNot:
		// NOT should be used with boolean expressions
		// This would require type inference in a full implementation
	}
}

func (v *Validator) validateLiteralValue(expr *LiteralExpression) {
	// Validate literal values are well-formed
	switch expr.Type {
	case LiteralString:
		if expr.Value == nil {
			v.addError("string literal cannot be null", expr)
		}
	case LiteralInteger:
		if _, ok := expr.Value.(int64); !ok {
			v.addError("invalid integer literal", expr)
		}
	case LiteralFloat:
		if _, ok := expr.Value.(float64); !ok {
			v.addError("invalid float literal", expr)
		}
	case LiteralBoolean:
		if _, ok := expr.Value.(bool); !ok {
			v.addError("invalid boolean literal", expr)
		}
	}
}

func (v *Validator) validateIdentifier(expr *IdentifierExpression) {
	// Check if identifier exists in current context
	fullName := expr.String()

	// Check in aliases first
	if _, exists := v.context.aliases[expr.Name]; exists {
		return
	}

	// Check in columns
	if _, exists := v.context.columns[fullName]; exists {
		return
	}

	// Check in tables if it's a simple identifier
	if expr.Table == "" && expr.Schema == "" {
		if _, exists := v.context.tables[expr.Name]; exists {
			return
		}
	}

	// If we get here, the identifier might not exist
	v.addWarning(fmt.Sprintf("identifier %s may not exist in current context", fullName), expr)
}

func (v *Validator) validateCaseResultTypes(expr *CaseExpression) {
	// In a full implementation, this would check that all WHEN and ELSE
	// result expressions have compatible types
	if len(expr.WhenClauses) == 0 {
		v.addError("CASE expression must have at least one WHEN clause", expr)
	}
}

func (v *Validator) expressionsEqual(expr1, expr2 Expression) bool {
	// Simplified expression equality check
	return expr1.String() == expr2.String()
}

// getBuiltinFunctions returns information about built-in SQL functions
func getBuiltinFunctions() map[string]*FunctionInfo {
	functions := make(map[string]*FunctionInfo)

	// Aggregate functions
	functions["COUNT"] = &FunctionInfo{Name: "COUNT", MinArgs: 0, MaxArgs: 1, IsAggregate: true, ReturnType: "INTEGER"}
	functions["SUM"] = &FunctionInfo{Name: "SUM", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "NUMERIC"}
	functions["AVG"] = &FunctionInfo{Name: "AVG", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "NUMERIC"}
	functions["MIN"] = &FunctionInfo{Name: "MIN", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "ANY"}
	functions["MAX"] = &FunctionInfo{Name: "MAX", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "ANY"}
	functions["STDDEV"] = &FunctionInfo{Name: "STDDEV", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "NUMERIC"}
	functions["VARIANCE"] = &FunctionInfo{Name: "VARIANCE", MinArgs: 1, MaxArgs: 1, IsAggregate: true, ReturnType: "NUMERIC"}

	// Window functions
	functions["ROW_NUMBER"] = &FunctionInfo{Name: "ROW_NUMBER", MinArgs: 0, MaxArgs: 0, IsWindow: true, ReturnType: "INTEGER"}
	functions["RANK"] = &FunctionInfo{Name: "RANK", MinArgs: 0, MaxArgs: 0, IsWindow: true, ReturnType: "INTEGER"}
	functions["DENSE_RANK"] = &FunctionInfo{Name: "DENSE_RANK", MinArgs: 0, MaxArgs: 0, IsWindow: true, ReturnType: "INTEGER"}
	functions["LAG"] = &FunctionInfo{Name: "LAG", MinArgs: 1, MaxArgs: 3, IsWindow: true, ReturnType: "ANY"}
	functions["LEAD"] = &FunctionInfo{Name: "LEAD", MinArgs: 1, MaxArgs: 3, IsWindow: true, ReturnType: "ANY"}

	// String functions
	functions["UPPER"] = &FunctionInfo{Name: "UPPER", MinArgs: 1, MaxArgs: 1, ReturnType: "STRING"}
	functions["LOWER"] = &FunctionInfo{Name: "LOWER", MinArgs: 1, MaxArgs: 1, ReturnType: "STRING"}
	functions["LENGTH"] = &FunctionInfo{Name: "LENGTH", MinArgs: 1, MaxArgs: 1, ReturnType: "INTEGER"}
	functions["SUBSTRING"] = &FunctionInfo{Name: "SUBSTRING", MinArgs: 2, MaxArgs: 3, ReturnType: "STRING"}
	functions["CONCAT"] = &FunctionInfo{Name: "CONCAT", MinArgs: 2, MaxArgs: -1, ReturnType: "STRING"}

	// Math functions
	functions["ABS"] = &FunctionInfo{Name: "ABS", MinArgs: 1, MaxArgs: 1, ReturnType: "NUMERIC"}
	functions["ROUND"] = &FunctionInfo{Name: "ROUND", MinArgs: 1, MaxArgs: 2, ReturnType: "NUMERIC"}
	functions["FLOOR"] = &FunctionInfo{Name: "FLOOR", MinArgs: 1, MaxArgs: 1, ReturnType: "NUMERIC"}
	functions["CEIL"] = &FunctionInfo{Name: "CEIL", MinArgs: 1, MaxArgs: 1, ReturnType: "NUMERIC"}
	functions["SQRT"] = &FunctionInfo{Name: "SQRT", MinArgs: 1, MaxArgs: 1, ReturnType: "NUMERIC"}

	// Date/time functions
	functions["NOW"] = &FunctionInfo{Name: "NOW", MinArgs: 0, MaxArgs: 0, ReturnType: "TIMESTAMP"}
	functions["CURRENT_DATE"] = &FunctionInfo{Name: "CURRENT_DATE", MinArgs: 0, MaxArgs: 0, ReturnType: "DATE"}
	functions["CURRENT_TIME"] = &FunctionInfo{Name: "CURRENT_TIME", MinArgs: 0, MaxArgs: 0, ReturnType: "TIME"}
	functions["EXTRACT"] = &FunctionInfo{Name: "EXTRACT", MinArgs: 2, MaxArgs: 2, ReturnType: "INTEGER"}

	// Conditional functions
	functions["COALESCE"] = &FunctionInfo{Name: "COALESCE", MinArgs: 2, MaxArgs: -1, ReturnType: "ANY"}
	functions["NULLIF"] = &FunctionInfo{Name: "NULLIF", MinArgs: 2, MaxArgs: 2, ReturnType: "ANY"}

	return functions
}

// Transaction statement visitor methods

func (v *Validator) VisitBeginTransactionStatement(stmt *BeginTransactionStatement) interface{} {
	// Validate isolation level if specified
	if stmt.IsolationLevel != nil {
		switch *stmt.IsolationLevel {
		case SQLReadUncommitted, SQLReadCommitted, SQLRepeatableRead, SQLSerializable:
			// Valid isolation levels
		default:
			v.addError("invalid isolation level", stmt)
		}
	}
	return nil
}

func (v *Validator) VisitCommitTransactionStatement(stmt *CommitTransactionStatement) interface{} {
	// No specific validation needed for COMMIT
	return nil
}

func (v *Validator) VisitRollbackTransactionStatement(stmt *RollbackTransactionStatement) interface{} {
	// Validate savepoint name if specified
	if stmt.Savepoint != "" {
		if !isValidIdentifier(stmt.Savepoint) {
			v.addError("invalid savepoint name", stmt)
		}
	}
	return nil
}

func (v *Validator) VisitSavepointStatement(stmt *SavepointStatement) interface{} {
	// Validate savepoint name
	if !isValidIdentifier(stmt.Name) {
		v.addError("invalid savepoint name", stmt)
	}
	return nil
}

func (v *Validator) VisitReleaseSavepointStatement(stmt *ReleaseSavepointStatement) interface{} {
	// Validate savepoint name
	if !isValidIdentifier(stmt.Name) {
		v.addError("invalid savepoint name", stmt)
	}
	return nil
}

// Helper function to validate identifiers
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}

	// Simple validation - starts with letter or underscore, contains only alphanumeric and underscore
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}

	for i := 1; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}
