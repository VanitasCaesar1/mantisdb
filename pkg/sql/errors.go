package sql

import (
	"fmt"
	"strings"
)

// SQLError represents a comprehensive SQL error with context
type SQLError struct {
	Type       ErrorType
	Message    string
	Position   int
	Line       int
	Column     int
	Token      string
	Query      string
	Suggestion string
	Context    string
}

// ErrorType represents the type of SQL error
type ErrorType int

const (
	SyntaxError ErrorType = iota
	SemanticError
	SQLValidationError
	RuntimeError
)

func (et ErrorType) String() string {
	switch et {
	case SyntaxError:
		return "Syntax Error"
	case SemanticError:
		return "Semantic Error"
	case SQLValidationError:
		return "Validation Error"
	case RuntimeError:
		return "Runtime Error"
	default:
		return "Unknown Error"
	}
}

func (e *SQLError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s: %s", e.Type, e.Message))

	if e.Line > 0 && e.Column > 0 {
		parts = append(parts, fmt.Sprintf("at line %d, column %d", e.Line, e.Column))
	}

	if e.Token != "" {
		parts = append(parts, fmt.Sprintf("near '%s'", e.Token))
	}

	if e.Context != "" {
		parts = append(parts, fmt.Sprintf("in context: %s", e.Context))
	}

	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("suggestion: %s", e.Suggestion))
	}

	return strings.Join(parts, " ")
}

// NewSyntaxError creates a new syntax error
func NewSyntaxError(message string, position, line, column int, token string) *SQLError {
	return &SQLError{
		Type:     SyntaxError,
		Message:  message,
		Position: position,
		Line:     line,
		Column:   column,
		Token:    token,
	}
}

// NewSemanticError creates a new semantic error
func NewSemanticError(message string, context string) *SQLError {
	return &SQLError{
		Type:    SemanticError,
		Message: message,
		Context: context,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(message string, suggestion string) *SQLError {
	return &SQLError{
		Type:       SQLValidationError,
		Message:    message,
		Suggestion: suggestion,
	}
}

// ErrorRecovery provides error recovery suggestions
type ErrorRecovery struct {
	commonMistakes map[string]string
	suggestions    map[string][]string
}

// NewErrorRecovery creates a new error recovery helper
func NewErrorRecovery() *ErrorRecovery {
	return &ErrorRecovery{
		commonMistakes: map[string]string{
			"SELCT":     "SELECT",
			"FORM":      "FROM",
			"WHRE":      "WHERE",
			"GROPU":     "GROUP",
			"OERDER":    "ORDER",
			"HAIVNG":    "HAVING",
			"JION":      "JOIN",
			"INNRE":     "INNER",
			"LEFFT":     "LEFT",
			"RIHGT":     "RIGHT",
			"CRETE":     "CREATE",
			"TALBE":     "TABLE",
			"INDX":      "INDEX",
			"PRIMRAY":   "PRIMARY",
			"FOREGN":    "FOREIGN",
			"REFERNCES": "REFERENCES",
			"UNIQU":     "UNIQUE",
			"DEFALT":    "DEFAULT",
			"NULABLE":   "NULLABLE",
			"INTEGR":    "INTEGER",
			"VARCAHR":   "VARCHAR",
			"TIMESTMP":  "TIMESTAMP",
		},
		suggestions: map[string][]string{
			"missing_from": {
				"Add a FROM clause to specify the table(s) to query",
				"Example: SELECT * FROM table_name",
			},
			"missing_select": {
				"Add a SELECT clause to specify the columns to retrieve",
				"Example: SELECT column1, column2 FROM table_name",
			},
			"invalid_join": {
				"Check JOIN syntax: table1 JOIN table2 ON condition",
				"Ensure ON or USING clause is present for JOIN",
			},
			"missing_paren": {
				"Check for missing parentheses in expressions",
				"Ensure all opening parentheses have matching closing ones",
			},
			"invalid_operator": {
				"Check operator syntax and spacing",
				"Common operators: =, <>, <, >, <=, >=, LIKE, IN, EXISTS",
			},
		},
	}
}

// SuggestCorrection suggests a correction for a misspelled keyword
func (er *ErrorRecovery) SuggestCorrection(word string) string {
	if correction, exists := er.commonMistakes[strings.ToUpper(word)]; exists {
		return correction
	}

	// Simple Levenshtein distance-based suggestion
	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER",
		"TABLE", "INDEX", "VIEW", "DATABASE", "SCHEMA", "COLUMN", "CONSTRAINT",
		"PRIMARY", "FOREIGN", "UNIQUE", "CHECK", "DEFAULT", "NOT", "NULL",
		"JOIN", "INNER", "LEFT", "RIGHT", "FULL", "OUTER", "CROSS", "NATURAL",
		"GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "DISTINCT", "ALL", "ANY", "SOME",
		"UNION", "INTERSECT", "EXCEPT", "WITH", "RECURSIVE", "AS",
		"CASE", "WHEN", "THEN", "ELSE", "END", "IF", "ELSEIF", "WHILE", "FOR",
		"INTEGER", "VARCHAR", "CHAR", "TEXT", "DATE", "TIME", "TIMESTAMP", "BOOLEAN",
		"AND", "OR", "IN", "EXISTS", "BETWEEN", "LIKE", "IS",
	}

	bestMatch := ""
	minDistance := len(word) + 1

	for _, keyword := range keywords {
		distance := levenshteinDistance(strings.ToUpper(word), keyword)
		if distance < minDistance && distance <= 2 { // Allow up to 2 character differences
			minDistance = distance
			bestMatch = keyword
		}
	}

	return bestMatch
}

// GetSuggestions returns suggestions for a given error type
func (er *ErrorRecovery) GetSuggestions(errorType string) []string {
	if suggestions, exists := er.suggestions[errorType]; exists {
		return suggestions
	}
	return []string{}
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}

	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// EnhancedParser extends the basic parser with better error reporting
type EnhancedParser struct {
	*Parser
	recovery *ErrorRecovery
	query    string
}

// NewEnhancedParser creates a new enhanced parser with error recovery
func NewEnhancedParser(tokens []Token, query string) *EnhancedParser {
	return &EnhancedParser{
		Parser:   NewParser(tokens),
		recovery: NewErrorRecovery(),
		query:    query,
	}
}

// enhancedError creates an enhanced error with suggestions
func (ep *EnhancedParser) enhancedError(message string) error {
	sqlErr := &SQLError{
		Type:     SyntaxError,
		Message:  message,
		Position: ep.current.Position,
		Line:     ep.current.Line,
		Column:   ep.current.Column,
		Token:    ep.current.Value,
		Query:    ep.query,
	}

	// Add suggestions based on the error
	if strings.Contains(message, "expected") {
		if strings.Contains(message, "FROM") {
			sqlErr.Suggestion = "Add a FROM clause to specify the source table"
		} else if strings.Contains(message, "SELECT") {
			sqlErr.Suggestion = "Add a SELECT clause to specify the columns to retrieve"
		} else if strings.Contains(message, "')'") {
			sqlErr.Suggestion = "Check for missing closing parenthesis"
		} else if strings.Contains(message, "'('") {
			sqlErr.Suggestion = "Check for missing opening parenthesis"
		}
	}

	// Check for common misspellings
	if ep.current.Type == TokenIdentifier {
		correction := ep.recovery.SuggestCorrection(ep.current.Value)
		if correction != "" {
			sqlErr.Suggestion = fmt.Sprintf("Did you mean '%s'?", correction)
		}
	}

	return sqlErr
}

// ParseSQLEnhanced parses SQL with enhanced error reporting
func ParseSQLEnhanced(input string) (Statement, error) {
	tokens, err := TokenizeSQL(input)
	if err != nil {
		return nil, err
	}

	parser := NewEnhancedParser(tokens, input)

	// For now, we'll use the standard parser and enhance errors post-parsing
	// In a full implementation, we'd modify the parser to use enhanced error reporting
	stmt, parseErr := parser.ParseStatement()

	if parseErr != nil {
		// Convert standard parse error to enhanced error
		if pe, ok := parseErr.(*ParseError); ok {
			return nil, &SQLError{
				Type:     SyntaxError,
				Message:  pe.Message,
				Position: pe.Position,
				Line:     pe.Line,
				Column:   pe.Column,
				Token:    pe.Token,
				Query:    input,
			}
		}
		return nil, parseErr
	}

	return stmt, nil
}
