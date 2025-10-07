package sql

import (
	"fmt"
	"strings"
	"unicode"
)

// Token represents a lexical token in SQL
type Token struct {
	Type     TokenType
	Value    string
	Position int
	Line     int
	Column   int
}

// TokenType represents the type of a token
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError

	// Literals
	TokenString
	TokenInteger
	TokenFloat
	TokenBoolean
	TokenNull

	// Identifiers and keywords
	TokenIdentifier
	TokenKeyword
	TokenQuotedIdentifier

	// Operators
	TokenEqual
	TokenNotEqual
	TokenLess
	TokenLessEqual
	TokenGreater
	TokenGreaterEqual
	TokenLike
	TokenNotLike
	TokenILike
	TokenNotILike
	TokenIn
	TokenNotIn
	TokenExists
	TokenNotExists
	TokenAnd
	TokenOr
	TokenNot
	TokenPlus
	TokenMinus
	TokenMultiply
	TokenDivide
	TokenModulo
	TokenConcat
	TokenBitwiseAnd
	TokenBitwiseOr
	TokenBitwiseXor
	TokenBitwiseNot
	TokenLeftShift
	TokenRightShift
	TokenRegexMatch
	TokenRegexNotMatch
	TokenJsonExtract
	TokenJsonExtractText

	// Punctuation
	TokenLeftParen
	TokenRightParen
	TokenLeftBracket
	TokenRightBracket
	TokenLeftBrace
	TokenRightBrace
	TokenComma
	TokenSemicolon
	TokenDot
	TokenDoubleColon
	TokenQuestion
	TokenDollar

	// Assignment
	TokenAssign
)

// String returns the string representation of a token type
func (tt TokenType) String() string {
	switch tt {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "ERROR"
	case TokenString:
		return "STRING"
	case TokenInteger:
		return "INTEGER"
	case TokenFloat:
		return "FLOAT"
	case TokenBoolean:
		return "BOOLEAN"
	case TokenNull:
		return "NULL"
	case TokenIdentifier:
		return "IDENTIFIER"
	case TokenKeyword:
		return "KEYWORD"
	case TokenQuotedIdentifier:
		return "QUOTED_IDENTIFIER"
	case TokenEqual:
		return "="
	case TokenNotEqual:
		return "!="
	case TokenLess:
		return "<"
	case TokenLessEqual:
		return "<="
	case TokenGreater:
		return ">"
	case TokenGreaterEqual:
		return ">="
	case TokenLike:
		return "LIKE"
	case TokenNotLike:
		return "NOT LIKE"
	case TokenILike:
		return "ILIKE"
	case TokenNotILike:
		return "NOT ILIKE"
	case TokenIn:
		return "IN"
	case TokenNotIn:
		return "NOT IN"
	case TokenExists:
		return "EXISTS"
	case TokenNotExists:
		return "NOT EXISTS"
	case TokenAnd:
		return "AND"
	case TokenOr:
		return "OR"
	case TokenNot:
		return "NOT"
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenMultiply:
		return "*"
	case TokenDivide:
		return "/"
	case TokenModulo:
		return "%"
	case TokenConcat:
		return "||"
	case TokenBitwiseAnd:
		return "&"
	case TokenBitwiseOr:
		return "|"
	case TokenBitwiseXor:
		return "^"
	case TokenBitwiseNot:
		return "~"
	case TokenLeftShift:
		return "<<"
	case TokenRightShift:
		return ">>"
	case TokenRegexMatch:
		return "~"
	case TokenRegexNotMatch:
		return "!~"
	case TokenJsonExtract:
		return "->"
	case TokenJsonExtractText:
		return "->>"
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	case TokenLeftBracket:
		return "["
	case TokenRightBracket:
		return "]"
	case TokenLeftBrace:
		return "{"
	case TokenRightBrace:
		return "}"
	case TokenComma:
		return ","
	case TokenSemicolon:
		return ";"
	case TokenDot:
		return "."
	case TokenDoubleColon:
		return "::"
	case TokenQuestion:
		return "?"
	case TokenDollar:
		return "$"
	case TokenAssign:
		return ":="
	default:
		return "UNKNOWN"
	}
}

// Lexer tokenizes SQL input - simplified implementation based on PostgreSQL's approach
type Lexer struct {
	input string
	pos   int
	line  int
	col   int
}

// NewLexer creates a new SQL lexer
func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Position: l.pos, Line: l.line, Column: l.col}
	}

	start := l.pos
	startLine := l.line
	startCol := l.col

	ch := l.input[l.pos]

	switch {
	case ch == '\'':
		return l.scanString(start, startLine, startCol)
	case ch == '"' || ch == '`':
		return l.scanQuotedIdentifier(start, startLine, startCol)
	case unicode.IsDigit(rune(ch)):
		return l.scanNumber(start, startLine, startCol)
	case unicode.IsLetter(rune(ch)) || ch == '_':
		return l.scanIdentifier(start, startLine, startCol)
	case ch == '-' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '-':
		l.skipLineComment()
		return l.NextToken()
	case ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*':
		l.skipBlockComment()
		return l.NextToken()
	default:
		return l.scanOperator(start, startLine, startCol)
	}
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

// skipLineComment skips a line comment (-- comment)
func (l *Lexer) skipLineComment() {
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.pos++
		l.col++
	}
}

// skipBlockComment skips a block comment (/* comment */)
func (l *Lexer) skipBlockComment() {
	l.pos += 2 // skip /*
	l.col += 2

	for l.pos+1 < len(l.input) {
		if l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {
			l.pos += 2
			l.col += 2
			break
		}
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

// scanString scans a string literal
func (l *Lexer) scanString(start, startLine, startCol int) Token {
	l.pos++ // skip opening quote
	l.col++

	for l.pos < len(l.input) && l.input[l.pos] != '\'' {
		if l.input[l.pos] == '\\' && l.pos+1 < len(l.input) {
			l.pos += 2 // skip escape sequence
			l.col += 2
		} else {
			if l.input[l.pos] == '\n' {
				l.line++
				l.col = 1
			} else {
				l.col++
			}
			l.pos++
		}
	}

	if l.pos >= len(l.input) {
		return Token{
			Type:     TokenError,
			Value:    "unterminated string literal",
			Position: start,
			Line:     startLine,
			Column:   startCol,
		}
	}

	l.pos++ // skip closing quote
	l.col++

	value := l.input[start+1 : l.pos-1] // exclude quotes
	return Token{
		Type:     TokenString,
		Value:    value,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

// scanQuotedIdentifier scans a quoted identifier
func (l *Lexer) scanQuotedIdentifier(start, startLine, startCol int) Token {
	quote := l.input[l.pos]
	l.pos++ // skip opening quote
	l.col++

	for l.pos < len(l.input) && l.input[l.pos] != quote {
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}

	if l.pos >= len(l.input) {
		return Token{
			Type:     TokenError,
			Value:    "unterminated quoted identifier",
			Position: start,
			Line:     startLine,
			Column:   startCol,
		}
	}

	l.pos++ // skip closing quote
	l.col++

	value := l.input[start+1 : l.pos-1] // exclude quotes
	return Token{
		Type:     TokenQuotedIdentifier,
		Value:    value,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

// scanNumber scans a numeric literal
func (l *Lexer) scanNumber(start, startLine, startCol int) Token {
	hasDecimal := false

	for l.pos < len(l.input) && unicode.IsDigit(rune(l.input[l.pos])) {
		l.pos++
		l.col++
	}

	if l.pos < len(l.input) && l.input[l.pos] == '.' &&
		l.pos+1 < len(l.input) && unicode.IsDigit(rune(l.input[l.pos+1])) {
		hasDecimal = true
		l.pos++ // skip '.'
		l.col++
		for l.pos < len(l.input) && unicode.IsDigit(rune(l.input[l.pos])) {
			l.pos++
			l.col++
		}
	}

	// Handle scientific notation
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		hasDecimal = true
		l.pos++
		l.col++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
			l.col++
		}
		for l.pos < len(l.input) && unicode.IsDigit(rune(l.input[l.pos])) {
			l.pos++
			l.col++
		}
	}

	value := l.input[start:l.pos]
	tokenType := TokenInteger
	if hasDecimal {
		tokenType = TokenFloat
	}

	return Token{
		Type:     tokenType,
		Value:    value,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

// scanIdentifier scans an identifier or keyword
func (l *Lexer) scanIdentifier(start, startLine, startCol int) Token {
	for l.pos < len(l.input) &&
		(unicode.IsLetter(rune(l.input[l.pos])) ||
			unicode.IsDigit(rune(l.input[l.pos])) ||
			l.input[l.pos] == '_') {
		l.pos++
		l.col++
	}

	value := l.input[start:l.pos]
	tokenType := TokenIdentifier

	if isKeyword(strings.ToUpper(value)) {
		tokenType = TokenKeyword

		// Handle special keywords that map to specific token types
		switch strings.ToUpper(value) {
		case "TRUE", "FALSE":
			tokenType = TokenBoolean
		case "NULL":
			tokenType = TokenNull
		case "AND":
			tokenType = TokenAnd
		case "OR":
			tokenType = TokenOr
		case "NOT":
			tokenType = TokenNot
		case "LIKE":
			tokenType = TokenLike
		case "ILIKE":
			tokenType = TokenILike
		case "IN":
			tokenType = TokenIn
		case "EXISTS":
			tokenType = TokenExists
		}
	}

	return Token{
		Type:     tokenType,
		Value:    value,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

// scanOperator scans operators and punctuation
func (l *Lexer) scanOperator(start, startLine, startCol int) Token {
	ch := l.input[l.pos]

	switch ch {
	case '(':
		l.pos++
		l.col++
		return Token{Type: TokenLeftParen, Value: "(", Position: start, Line: startLine, Column: startCol}
	case ')':
		l.pos++
		l.col++
		return Token{Type: TokenRightParen, Value: ")", Position: start, Line: startLine, Column: startCol}
	case '[':
		l.pos++
		l.col++
		return Token{Type: TokenLeftBracket, Value: "[", Position: start, Line: startLine, Column: startCol}
	case ']':
		l.pos++
		l.col++
		return Token{Type: TokenRightBracket, Value: "]", Position: start, Line: startLine, Column: startCol}
	case '{':
		l.pos++
		l.col++
		return Token{Type: TokenLeftBrace, Value: "{", Position: start, Line: startLine, Column: startCol}
	case '}':
		l.pos++
		l.col++
		return Token{Type: TokenRightBrace, Value: "}", Position: start, Line: startLine, Column: startCol}
	case ',':
		l.pos++
		l.col++
		return Token{Type: TokenComma, Value: ",", Position: start, Line: startLine, Column: startCol}
	case ';':
		l.pos++
		l.col++
		return Token{Type: TokenSemicolon, Value: ";", Position: start, Line: startLine, Column: startCol}
	case '.':
		l.pos++
		l.col++
		return Token{Type: TokenDot, Value: ".", Position: start, Line: startLine, Column: startCol}
	case '?':
		l.pos++
		l.col++
		return Token{Type: TokenQuestion, Value: "?", Position: start, Line: startLine, Column: startCol}
	case '$':
		l.pos++
		l.col++
		return Token{Type: TokenDollar, Value: "$", Position: start, Line: startLine, Column: startCol}
	case '+':
		l.pos++
		l.col++
		return Token{Type: TokenPlus, Value: "+", Position: start, Line: startLine, Column: startCol}
	case '*':
		l.pos++
		l.col++
		return Token{Type: TokenMultiply, Value: "*", Position: start, Line: startLine, Column: startCol}
	case '/':
		l.pos++
		l.col++
		return Token{Type: TokenDivide, Value: "/", Position: start, Line: startLine, Column: startCol}
	case '%':
		l.pos++
		l.col++
		return Token{Type: TokenModulo, Value: "%", Position: start, Line: startLine, Column: startCol}
	case '&':
		l.pos++
		l.col++
		return Token{Type: TokenBitwiseAnd, Value: "&", Position: start, Line: startLine, Column: startCol}
	case '^':
		l.pos++
		l.col++
		return Token{Type: TokenBitwiseXor, Value: "^", Position: start, Line: startLine, Column: startCol}
	case '~':
		l.pos++
		l.col++
		return Token{Type: TokenBitwiseNot, Value: "~", Position: start, Line: startLine, Column: startCol}
	case '=':
		l.pos++
		l.col++
		return Token{Type: TokenEqual, Value: "=", Position: start, Line: startLine, Column: startCol}
	case '-':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == '>' {
			l.pos++
			l.col++
			if l.pos < len(l.input) && l.input[l.pos] == '>' {
				l.pos++
				l.col++
				return Token{Type: TokenJsonExtractText, Value: "->>", Position: start, Line: startLine, Column: startCol}
			}
			return Token{Type: TokenJsonExtract, Value: "->", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenMinus, Value: "-", Position: start, Line: startLine, Column: startCol}
	case '<':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			l.col++
			return Token{Type: TokenLessEqual, Value: "<=", Position: start, Line: startLine, Column: startCol}
		} else if l.pos < len(l.input) && l.input[l.pos] == '>' {
			l.pos++
			l.col++
			return Token{Type: TokenNotEqual, Value: "<>", Position: start, Line: startLine, Column: startCol}
		} else if l.pos < len(l.input) && l.input[l.pos] == '<' {
			l.pos++
			l.col++
			return Token{Type: TokenLeftShift, Value: "<<", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenLess, Value: "<", Position: start, Line: startLine, Column: startCol}
	case '>':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			l.col++
			return Token{Type: TokenGreaterEqual, Value: ">=", Position: start, Line: startLine, Column: startCol}
		} else if l.pos < len(l.input) && l.input[l.pos] == '>' {
			l.pos++
			l.col++
			return Token{Type: TokenRightShift, Value: ">>", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenGreater, Value: ">", Position: start, Line: startLine, Column: startCol}
	case '!':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			l.col++
			return Token{Type: TokenNotEqual, Value: "!=", Position: start, Line: startLine, Column: startCol}
		} else if l.pos < len(l.input) && l.input[l.pos] == '~' {
			l.pos++
			l.col++
			return Token{Type: TokenRegexNotMatch, Value: "!~", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenError, Value: "unexpected character '!'", Position: start, Line: startLine, Column: startCol}
	case '|':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == '|' {
			l.pos++
			l.col++
			return Token{Type: TokenConcat, Value: "||", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenBitwiseOr, Value: "|", Position: start, Line: startLine, Column: startCol}
	case ':':
		l.pos++
		l.col++
		if l.pos < len(l.input) && l.input[l.pos] == ':' {
			l.pos++
			l.col++
			return Token{Type: TokenDoubleColon, Value: "::", Position: start, Line: startLine, Column: startCol}
		} else if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.pos++
			l.col++
			return Token{Type: TokenAssign, Value: ":=", Position: start, Line: startLine, Column: startCol}
		}
		return Token{Type: TokenError, Value: "unexpected character ':'", Position: start, Line: startLine, Column: startCol}
	default:
		l.pos++
		l.col++
		return Token{
			Type:     TokenError,
			Value:    fmt.Sprintf("unexpected character '%c'", ch),
			Position: start,
			Line:     startLine,
			Column:   startCol,
		}
	}
}

// isKeyword checks if a string is a SQL keyword
func isKeyword(s string) bool {
	keywords := map[string]bool{
		// Basic SQL keywords
		"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "INTO": true,
		"VALUES": true, "UPDATE": true, "SET": true, "DELETE": true, "CREATE": true,
		"DROP": true, "ALTER": true, "TABLE": true, "INDEX": true, "VIEW": true,
		"DATABASE": true, "SCHEMA": true, "COLUMN": true, "CONSTRAINT": true,

		// Data types
		"INTEGER": true, "INT": true, "BIGINT": true, "SMALLINT": true, "TINYINT": true,
		"DECIMAL": true, "NUMERIC": true, "FLOAT": true, "REAL": true, "DOUBLE": true,
		"VARCHAR": true, "CHAR": true, "TEXT": true, "BLOB": true, "CLOB": true,
		"DATE": true, "TIME": true, "TIMESTAMP": true, "DATETIME": true, "INTERVAL": true,
		"BOOLEAN": true, "BOOL": true, "JSON": true, "JSONB": true, "XML": true,
		"UUID": true, "ARRAY": true, "SERIAL": true, "BIGSERIAL": true,

		// Constraints
		"PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true,
		"UNIQUE": true, "CHECK": true, "DEFAULT": true, "NOT": true, "NULL": true,
		"AUTO_INCREMENT": true, "IDENTITY": true, "GENERATED": true, "ALWAYS": true,

		// Operators and functions
		"AND": true, "OR": true, "IN": true, "EXISTS": true, "BETWEEN": true,
		"LIKE": true, "ILIKE": true, "SIMILAR": true, "REGEXP": true, "RLIKE": true,
		"IS": true, "DISTINCT": true, "ALL": true, "ANY": true, "SOME": true,

		// Joins
		"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
		"OUTER": true, "CROSS": true, "NATURAL": true, "ON": true, "USING": true,

		// Grouping and ordering
		"GROUP": true, "BY": true, "HAVING": true, "ORDER": true, "ASC": true,
		"DESC": true, "LIMIT": true, "OFFSET": true, "FETCH": true, "FIRST": true,
		"LAST": true, "ROWS": true, "ONLY": true, "NULLS": true,

		// Window functions
		"OVER": true, "PARTITION": true, "RANGE": true, "UNBOUNDED": true,
		"PRECEDING": true, "FOLLOWING": true, "CURRENT": true, "ROW": true,
		"FILTER": true,

		// Set operations
		"UNION": true, "INTERSECT": true, "EXCEPT": true, "MINUS": true,

		// Common Table Expressions
		"WITH": true, "RECURSIVE": true, "AS": true,

		// Case expressions
		"CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true,

		// Transactions
		"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
		"START": true, "SAVEPOINT": true, "RELEASE": true,

		// Access control
		"GRANT": true, "REVOKE": true, "ROLE": true, "USER": true, "PRIVILEGE": true,
		"PRIVILEGES": true, "PUBLIC": true,

		// Procedural
		"IF": true, "ELSEIF": true, "WHILE": true, "FOR": true, "LOOP": true,
		"REPEAT": true, "UNTIL": true, "RETURN": true, "CALL": true, "FUNCTION": true,
		"PROCEDURE": true, "TRIGGER": true, "DECLARE": true,

		// Literals
		"TRUE": true, "FALSE": true,

		// Conflict resolution
		"CONFLICT": true, "DO": true, "NOTHING": true, "REPLACE": true,
		"IGNORE": true, "UPSERT": true,

		// DDL specific keywords
		"ADD": true, "MODIFY": true, "CHANGE": true, "RENAME": true, "TO": true,
		"AFTER": true, "BEFORE": true,

		// Advanced SQL features
		"MATERIALIZED": true, "REFRESH": true, "CONCURRENTLY": true,
		"EXPLAIN": true, "ANALYZE": true, "VERBOSE": true,
		"LATERAL": true, "TABLESAMPLE": true, "BERNOULLI": true, "SYSTEM": true,

		// Misc
		"TEMPORARY": true, "TEMP": true, "CASCADE": true, "RESTRICT": true,
		"MATCH": true, "PARTIAL": true, "SIMPLE": true,
		"ACTION": true, "NO": true, "DEFERRABLE": true, "INITIALLY": true,
		"DEFERRED": true, "IMMEDIATE": true,
	}

	return keywords[s]
}

// TokenizeSQL tokenizes a SQL string and returns all tokens
func TokenizeSQL(input string) ([]Token, error) {
	lexer := NewLexer(input)
	var tokens []Token

	for {
		token := lexer.NextToken()
		if token.Type == TokenError {
			return nil, fmt.Errorf("lexer error: %s", token.Value)
		}

		tokens = append(tokens, token)

		if token.Type == TokenEOF {
			break
		}
	}

	return tokens, nil
}
