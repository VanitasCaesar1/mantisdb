package query

import (
	"errors"
	"fmt"
	"strings"
)

// QueryType represents the type of query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeInsert
	QueryTypeUpdate
	QueryTypeDelete
	QueryTypeCreate
	QueryTypeDrop
)

// Query represents a parsed query
type Query struct {
	Type       QueryType
	Table      string
	Fields     []string
	Values     map[string]interface{}
	Conditions []Condition
	OrderBy    []OrderByClause
	Limit      int
	Offset     int
}

// Condition represents a WHERE condition
type Condition struct {
	Field    string
	Operator string
	Value    interface{}
	Logic    string // AND, OR
}

// OrderByClause represents an ORDER BY clause
type OrderByClause struct {
	Field string
	Desc  bool
}

// Parser handles query parsing
type Parser struct {
	tokens []Token
	pos    int
}

// Token represents a lexical token
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}

// TokenType represents the type of token
type TokenType int

const (
	TokenKeyword TokenType = iota
	TokenIdentifier
	TokenString
	TokenNumber
	TokenOperator
	TokenPunctuation
	TokenEOF
)

// NewParser creates a new query parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a SQL-like query string
func (p *Parser) Parse(queryStr string) (*Query, error) {
	// Tokenize the query
	tokens, err := p.tokenize(queryStr)
	if err != nil {
		return nil, err
	}

	p.tokens = tokens
	p.pos = 0

	// Parse based on the first keyword
	if len(tokens) == 0 {
		return nil, errors.New("empty query")
	}

	firstToken := tokens[0]
	if firstToken.Type != TokenKeyword {
		return nil, errors.New("query must start with a keyword")
	}

	switch strings.ToUpper(firstToken.Value) {
	case "SELECT":
		return p.parseSelect()
	case "INSERT":
		return p.parseInsert()
	case "UPDATE":
		return p.parseUpdate()
	case "DELETE":
		return p.parseDelete()
	case "CREATE":
		return p.parseCreate()
	case "DROP":
		return p.parseDrop()
	default:
		return nil, fmt.Errorf("unsupported query type: %s", firstToken.Value)
	}
}

// tokenize breaks the query string into tokens
func (p *Parser) tokenize(queryStr string) ([]Token, error) {
	var tokens []Token
	queryStr = strings.TrimSpace(queryStr)

	i := 0
	for i < len(queryStr) {
		// Skip whitespace
		if isWhitespace(queryStr[i]) {
			i++
			continue
		}

		// String literals
		if queryStr[i] == '\'' || queryStr[i] == '"' {
			quote := queryStr[i]
			start := i + 1
			i++
			for i < len(queryStr) && queryStr[i] != quote {
				i++
			}
			if i >= len(queryStr) {
				return nil, errors.New("unterminated string literal")
			}
			tokens = append(tokens, Token{
				Type:  TokenString,
				Value: queryStr[start:i],
				Pos:   start,
			})
			i++
			continue
		}

		// Numbers
		if isDigit(queryStr[i]) {
			start := i
			for i < len(queryStr) && (isDigit(queryStr[i]) || queryStr[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{
				Type:  TokenNumber,
				Value: queryStr[start:i],
				Pos:   start,
			})
			continue
		}

		// Operators
		if isOperator(queryStr[i]) {
			start := i
			if i+1 < len(queryStr) && isTwoCharOperator(queryStr[i:i+2]) {
				i += 2
			} else {
				i++
			}
			tokens = append(tokens, Token{
				Type:  TokenOperator,
				Value: queryStr[start:i],
				Pos:   start,
			})
			continue
		}

		// Punctuation
		if isPunctuation(queryStr[i]) {
			tokens = append(tokens, Token{
				Type:  TokenPunctuation,
				Value: string(queryStr[i]),
				Pos:   i,
			})
			i++
			continue
		}

		// Identifiers and keywords
		if isAlpha(queryStr[i]) {
			start := i
			for i < len(queryStr) && (isAlphaNumeric(queryStr[i]) || queryStr[i] == '_') {
				i++
			}
			value := queryStr[start:i]
			tokenType := TokenIdentifier
			if isKeyword(value) {
				tokenType = TokenKeyword
			}
			tokens = append(tokens, Token{
				Type:  tokenType,
				Value: value,
				Pos:   start,
			})
			continue
		}

		return nil, fmt.Errorf("unexpected character: %c at position %d", queryStr[i], i)
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: "", Pos: len(queryStr)})
	return tokens, nil
}

// parseSelect parses a SELECT query
func (p *Parser) parseSelect() (*Query, error) {
	query := &Query{Type: QueryTypeSelect}

	// Skip SELECT keyword
	p.pos++

	// Parse fields
	fields, err := p.parseFieldList()
	if err != nil {
		return nil, err
	}
	query.Fields = fields

	// Parse FROM clause
	if !p.expectKeyword("FROM") {
		return nil, errors.New("expected FROM clause")
	}
	p.pos++

	table, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	query.Table = table

	// Parse optional clauses
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type != TokenEOF {
		token := p.tokens[p.pos]
		if token.Type != TokenKeyword {
			break
		}

		switch strings.ToUpper(token.Value) {
		case "WHERE":
			conditions, err := p.parseWhereClause()
			if err != nil {
				return nil, err
			}
			query.Conditions = conditions
		case "ORDER":
			orderBy, err := p.parseOrderByClause()
			if err != nil {
				return nil, err
			}
			query.OrderBy = orderBy
		case "LIMIT":
			limit, err := p.parseLimitClause()
			if err != nil {
				return nil, err
			}
			query.Limit = limit
		default:
			return nil, fmt.Errorf("unexpected keyword: %s", token.Value)
		}
	}

	return query, nil
}

// parseInsert parses an INSERT query
func (p *Parser) parseInsert() (*Query, error) {
	query := &Query{Type: QueryTypeInsert}

	// Skip INSERT keyword
	p.pos++

	// Expect INTO
	if !p.expectKeyword("INTO") {
		return nil, errors.New("expected INTO after INSERT")
	}
	p.pos++

	// Parse table name
	table, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	query.Table = table

	// Parse field list (optional)
	if p.pos < len(p.tokens) && p.tokens[p.pos].Value == "(" {
		p.pos++ // Skip opening parenthesis
		fields, err := p.parseFieldList()
		if err != nil {
			return nil, err
		}
		query.Fields = fields

		if !p.expectPunctuation(")") {
			return nil, errors.New("expected closing parenthesis")
		}
		p.pos++
	}

	// Parse VALUES clause
	if !p.expectKeyword("VALUES") {
		return nil, errors.New("expected VALUES clause")
	}
	p.pos++

	values, err := p.parseValuesList()
	if err != nil {
		return nil, err
	}
	query.Values = values

	return query, nil
}

// parseUpdate parses an UPDATE query
func (p *Parser) parseUpdate() (*Query, error) {
	query := &Query{Type: QueryTypeUpdate}

	// Skip UPDATE keyword
	p.pos++

	// Parse table name
	table, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	query.Table = table

	// Parse SET clause
	if !p.expectKeyword("SET") {
		return nil, errors.New("expected SET clause")
	}
	p.pos++

	values, err := p.parseSetClause()
	if err != nil {
		return nil, err
	}
	query.Values = values

	// Parse optional WHERE clause
	if p.pos < len(p.tokens) && p.expectKeyword("WHERE") {
		conditions, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		query.Conditions = conditions
	}

	return query, nil
}

// parseDelete parses a DELETE query
func (p *Parser) parseDelete() (*Query, error) {
	query := &Query{Type: QueryTypeDelete}

	// Skip DELETE keyword
	p.pos++

	// Parse FROM clause
	if !p.expectKeyword("FROM") {
		return nil, errors.New("expected FROM clause")
	}
	p.pos++

	table, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	query.Table = table

	// Parse optional WHERE clause
	if p.pos < len(p.tokens) && p.expectKeyword("WHERE") {
		conditions, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		query.Conditions = conditions
	}

	return query, nil
}

// parseCreate parses a CREATE query (simplified)
func (p *Parser) parseCreate() (*Query, error) {
	return &Query{Type: QueryTypeCreate}, errors.New("CREATE queries not fully implemented")
}

// parseDrop parses a DROP query (simplified)
func (p *Parser) parseDrop() (*Query, error) {
	return &Query{Type: QueryTypeDrop}, errors.New("DROP queries not fully implemented")
}

// Helper methods for parsing specific clauses

func (p *Parser) parseFieldList() ([]string, error) {
	var fields []string

	for {
		if p.pos >= len(p.tokens) {
			break
		}

		token := p.tokens[p.pos]
		if token.Type == TokenOperator && token.Value == "*" {
			fields = append(fields, "*")
			p.pos++
		} else if token.Type == TokenIdentifier {
			fields = append(fields, token.Value)
			p.pos++
		} else {
			break
		}

		// Check for comma
		if p.pos < len(p.tokens) && p.tokens[p.pos].Value == "," {
			p.pos++
		} else {
			break
		}
	}

	return fields, nil
}

func (p *Parser) parseIdentifier() (string, error) {
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != TokenIdentifier {
		return "", errors.New("expected identifier")
	}

	identifier := p.tokens[p.pos].Value
	p.pos++
	return identifier, nil
}

func (p *Parser) parseWhereClause() ([]Condition, error) {
	// Skip WHERE keyword
	p.pos++

	var conditions []Condition

	for p.pos < len(p.tokens) {
		// Parse field
		field, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		// Parse operator
		if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != TokenOperator {
			return nil, errors.New("expected operator")
		}
		operator := p.tokens[p.pos].Value
		p.pos++

		// Parse value
		if p.pos >= len(p.tokens) {
			return nil, errors.New("expected value")
		}
		value := p.parseValue()

		condition := Condition{
			Field:    field,
			Operator: operator,
			Value:    value,
		}
		conditions = append(conditions, condition)

		// Check for AND/OR
		if p.pos < len(p.tokens) && p.tokens[p.pos].Type == TokenKeyword {
			logic := strings.ToUpper(p.tokens[p.pos].Value)
			if logic == "AND" || logic == "OR" {
				condition.Logic = logic
				p.pos++
			} else {
				break
			}
		} else {
			break
		}
	}

	return conditions, nil
}

func (p *Parser) parseOrderByClause() ([]OrderByClause, error) {
	// Skip ORDER keyword
	p.pos++

	// Expect BY
	if !p.expectKeyword("BY") {
		return nil, errors.New("expected BY after ORDER")
	}
	p.pos++

	var orderBy []OrderByClause

	for {
		field, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		clause := OrderByClause{Field: field}

		// Check for DESC
		if p.pos < len(p.tokens) && p.expectKeyword("DESC") {
			clause.Desc = true
			p.pos++
		} else if p.pos < len(p.tokens) && p.expectKeyword("ASC") {
			p.pos++
		}

		orderBy = append(orderBy, clause)

		// Check for comma
		if p.pos < len(p.tokens) && p.tokens[p.pos].Value == "," {
			p.pos++
		} else {
			break
		}
	}

	return orderBy, nil
}

func (p *Parser) parseLimitClause() (int, error) {
	// Skip LIMIT keyword
	p.pos++

	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != TokenNumber {
		return 0, errors.New("expected number after LIMIT")
	}

	// Simple conversion (in real implementation, use strconv)
	limit := 10 // Placeholder
	p.pos++

	return limit, nil
}

func (p *Parser) parseValuesList() (map[string]interface{}, error) {
	// Simplified implementation
	return make(map[string]interface{}), nil
}

func (p *Parser) parseSetClause() (map[string]interface{}, error) {
	// Simplified implementation
	return make(map[string]interface{}), nil
}

func (p *Parser) parseValue() interface{} {
	if p.pos >= len(p.tokens) {
		return nil
	}

	token := p.tokens[p.pos]
	p.pos++

	switch token.Type {
	case TokenString:
		return token.Value
	case TokenNumber:
		return token.Value // In real implementation, convert to appropriate numeric type
	default:
		return token.Value
	}
}

// Helper functions for token classification

func (p *Parser) expectKeyword(keyword string) bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	token := p.tokens[p.pos]
	return token.Type == TokenKeyword && strings.ToUpper(token.Value) == strings.ToUpper(keyword)
}

func (p *Parser) expectPunctuation(punct string) bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	token := p.tokens[p.pos]
	return token.Type == TokenPunctuation && token.Value == punct
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNumeric(c byte) bool {
	return isAlpha(c) || isDigit(c)
}

func isOperator(c byte) bool {
	return c == '=' || c == '<' || c == '>' || c == '!' || c == '*' || c == '+' || c == '-'
}

func isTwoCharOperator(s string) bool {
	return s == "<=" || s == ">=" || s == "!=" || s == "<>"
}

func isPunctuation(c byte) bool {
	return c == '(' || c == ')' || c == ',' || c == ';'
}

func isKeyword(s string) bool {
	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
		"DELETE", "CREATE", "DROP", "TABLE", "ORDER", "BY", "LIMIT", "OFFSET",
		"AND", "OR", "NOT", "ASC", "DESC",
	}

	upper := strings.ToUpper(s)
	for _, keyword := range keywords {
		if upper == keyword {
			return true
		}
	}
	return false
}
