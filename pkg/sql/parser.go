package sql

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser parses SQL statements into AST nodes
type Parser struct {
	tokens   []Token
	position int
	current  Token
}

// NewParser creates a new SQL parser
func NewParser(tokens []Token) *Parser {
	p := &Parser{
		tokens: tokens,
	}
	p.advance()
	return p
}

// ParseSQL parses a SQL string and returns the AST
func ParseSQL(input string) (Statement, error) {
	tokens, err := TokenizeSQL(input)
	if err != nil {
		return nil, err
	}

	parser := NewParser(tokens)
	return parser.ParseStatement()
}

// advance moves to the next token
func (p *Parser) advance() {
	if p.position < len(p.tokens) {
		p.current = p.tokens[p.position]
		p.position++
	} else {
		p.current = Token{Type: TokenEOF}
	}
}

// peek returns the next token without advancing
func (p *Parser) peek() Token {
	if p.position < len(p.tokens) {
		return p.tokens[p.position]
	}
	return Token{Type: TokenEOF}
}

// peekN returns the token at position + n without advancing
func (p *Parser) peekN(n int) Token {
	pos := p.position + n - 1
	if pos < len(p.tokens) {
		return p.tokens[pos]
	}
	return Token{Type: TokenEOF}
}

// match checks if current token matches any of the given types
func (p *Parser) match(types ...TokenType) bool {
	for _, t := range types {
		if p.current.Type == t {
			return true
		}
	}
	return false
}

// matchKeyword checks if current token is a keyword with given value
func (p *Parser) matchKeyword(keywords ...string) bool {
	if p.current.Type != TokenKeyword {
		return false
	}

	upper := strings.ToUpper(p.current.Value)
	for _, keyword := range keywords {
		if upper == strings.ToUpper(keyword) {
			return true
		}
	}
	return false
}

// consume advances if current token matches, otherwise returns error
func (p *Parser) consume(tokenType TokenType, message string) error {
	if p.current.Type == tokenType {
		p.advance()
		return nil
	}
	return p.error(message)
}

// consumeKeyword advances if current token is the expected keyword
func (p *Parser) consumeKeyword(keyword, message string) error {
	if p.matchKeyword(keyword) {
		p.advance()
		return nil
	}
	return p.error(message)
}

// error creates a parse error
func (p *Parser) error(message string) error {
	return &ParseError{
		Message:  message,
		Position: p.current.Position,
		Line:     p.current.Line,
		Column:   p.current.Column,
		Token:    p.current.Value,
	}
}

// ParseStatement parses a SQL statement
func (p *Parser) ParseStatement() (Statement, error) {
	// The lexer already skips whitespace and comments, so no need to skip here

	if !p.match(TokenKeyword) {
		return nil, p.error("expected SQL statement")
	}

	switch strings.ToUpper(p.current.Value) {
	case "SELECT", "WITH":
		return p.parseSelectStatement()
	case "INSERT":
		return p.parseInsertStatement()
	case "UPDATE":
		return p.parseUpdateStatement()
	case "DELETE":
		return p.parseDeleteStatement()
	case "CREATE":
		return p.parseCreateStatement()
	case "DROP":
		return p.parseDropStatement()
	case "ALTER":
		return p.parseAlterStatement()
	case "BEGIN", "START":
		return p.parseBeginTransactionStatement()
	case "COMMIT":
		return p.parseCommitTransactionStatement()
	case "ROLLBACK":
		return p.parseRollbackTransactionStatement()
	case "SAVEPOINT":
		return p.parseSavepointStatement()
	case "RELEASE":
		return p.parseReleaseSavepointStatement()
	default:
		return nil, p.error(fmt.Sprintf("unsupported statement type: %s", p.current.Value))
	}
}

// parseSelectStatement parses a SELECT statement
func (p *Parser) parseSelectStatement() (*SelectStatement, error) {
	stmt := &SelectStatement{}

	// Parse WITH clause (Common Table Expressions)
	if p.matchKeyword("WITH") {
		ctes, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		stmt.With = ctes
	}

	// Parse SELECT
	if err := p.consumeKeyword("SELECT", "expected SELECT"); err != nil {
		return nil, err
	}

	// Parse DISTINCT
	if p.matchKeyword("DISTINCT") {
		stmt.Distinct = true
		p.advance()
	}

	// Parse select fields
	fields, err := p.parseSelectFields()
	if err != nil {
		return nil, err
	}
	stmt.Fields = fields

	// Parse FROM clause
	if p.matchKeyword("FROM") {
		p.advance()
		from, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}
		stmt.From = from
	}

	// Parse WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// Parse GROUP BY clause
	if p.matchKeyword("GROUP") {
		p.advance()
		if err := p.consumeKeyword("BY", "expected BY after GROUP"); err != nil {
			return nil, err
		}
		groupBy, err := p.parseExpressionList()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = groupBy
	}

	// Parse HAVING clause
	if p.matchKeyword("HAVING") {
		p.advance()
		having, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Having = having
	}

	// Parse window definitions
	if p.matchKeyword("WINDOW") {
		windows, err := p.parseWindowDefinitions()
		if err != nil {
			return nil, err
		}
		stmt.WindowDefs = windows
	}

	// Parse ORDER BY clause
	if p.matchKeyword("ORDER") {
		p.advance()
		if err := p.consumeKeyword("BY", "expected BY after ORDER"); err != nil {
			return nil, err
		}
		orderBy, err := p.parseOrderByClause()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// Parse LIMIT clause
	if p.matchKeyword("LIMIT") {
		p.advance()
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = &LimitClause{Count: limit}
	}

	// Parse OFFSET clause
	if p.matchKeyword("OFFSET") {
		p.advance()
		offset, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Offset = &OffsetClause{Count: offset}
	}

	return stmt, nil
}

// parseWithClause parses WITH clause (Common Table Expressions)
func (p *Parser) parseWithClause() ([]*CommonTableExpression, error) {
	p.advance() // consume WITH

	var ctes []*CommonTableExpression

	// Parse RECURSIVE if present
	recursive := false
	if p.matchKeyword("RECURSIVE") {
		recursive = true
		p.advance()
	}

	for {
		cte := &CommonTableExpression{}

		// Parse CTE name
		if !p.match(TokenIdentifier) {
			return nil, p.error("expected CTE name")
		}
		cte.Name = p.current.Value
		p.advance()

		// Parse optional column list
		if p.match(TokenLeftParen) {
			p.advance()
			for {
				if !p.match(TokenIdentifier) {
					return nil, p.error("expected column name")
				}
				cte.Columns = append(cte.Columns, p.current.Value)
				p.advance()

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}
			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
		}

		// Parse AS
		if err := p.consumeKeyword("AS", "expected AS"); err != nil {
			return nil, err
		}

		// Parse subquery
		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		query, err := p.parseSelectStatement()
		if err != nil {
			return nil, err
		}
		cte.Query = query

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}

		ctes = append(ctes, cte)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	// Mark as recursive if needed
	if recursive && len(ctes) > 0 {
		// In a real implementation, you'd mark the CTEs as recursive
		// For now, we'll just note that this was a recursive WITH
	}

	return ctes, nil
}

// parseSelectFields parses the SELECT field list
func (p *Parser) parseSelectFields() ([]SelectField, error) {
	var fields []SelectField

	for {
		field := SelectField{}

		// Parse expression
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		field.Expression = expr

		// Parse optional alias
		if p.matchKeyword("AS") {
			p.advance()
			if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
				return nil, p.error("expected alias name")
			}
			field.Alias = p.current.Value
			p.advance()
		} else if p.match(TokenIdentifier, TokenQuotedIdentifier) {
			// Implicit alias (identifier after expression without AS)
			field.Alias = p.current.Value
			p.advance()
		}

		fields = append(fields, field)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return fields, nil
}

// parseFromClause parses the FROM clause
func (p *Parser) parseFromClause() ([]TableReference, error) {
	var tables []TableReference

	for {
		table, err := p.parseTableReference()
		if err != nil {
			return nil, err
		}
		tables = append(tables, *table)

		// Check for JOINs
		for p.matchKeyword("JOIN", "INNER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL") {
			joinType, err := p.parseJoinType()
			if err != nil {
				return nil, err
			}

			joinTable, err := p.parseTableReference()
			if err != nil {
				return nil, err
			}

			join := &JoinClause{
				Type:  joinType,
				Table: joinTable,
			}

			// Parse join condition
			if p.matchKeyword("ON") {
				p.advance()
				condition, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				join.Condition = condition
			} else if p.matchKeyword("USING") {
				p.advance()
				if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
					return nil, err
				}

				for {
					if !p.match(TokenIdentifier) {
						return nil, p.error("expected column name")
					}
					join.Using = append(join.Using, p.current.Value)
					p.advance()

					if p.match(TokenComma) {
						p.advance()
					} else {
						break
					}
				}

				if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
					return nil, err
				}
			}

			// Add join to the last table
			if len(tables) > 0 {
				// In a real implementation, you'd properly handle the join structure
				// For now, we'll just note that there was a join
			}
		}

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return tables, nil
}

// parseTableReference parses a table reference
func (p *Parser) parseTableReference() (*TableReference, error) {
	table := &TableReference{}

	if p.match(TokenLeftParen) {
		// Subquery
		p.advance()
		subquery, err := p.parseSelectStatement()
		if err != nil {
			return nil, err
		}
		table.Subquery = subquery

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	} else {
		// Table name
		if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
			return nil, p.error("expected table name")
		}

		// Parse schema.table or just table
		name := p.current.Value
		p.advance()

		if p.match(TokenDot) {
			p.advance()
			if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
				return nil, p.error("expected table name after '.'")
			}
			table.Schema = name
			table.Name = p.current.Value
			p.advance()
		} else {
			table.Name = name
		}
	}

	// Parse optional LATERAL (for subqueries)
	lateral := false
	if p.matchKeyword("LATERAL") {
		lateral = true
		p.advance()

		if p.match(TokenLeftParen) {
			// LATERAL subquery
			p.advance()
			subquery, err := p.parseSelectStatement()
			if err != nil {
				return nil, err
			}
			table.Subquery = subquery

			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
		}
	}

	// Parse optional alias
	if p.matchKeyword("AS") {
		p.advance()
		if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
			return nil, p.error("expected alias name")
		}
		table.Alias = p.current.Value
		p.advance()
	} else if p.match(TokenIdentifier, TokenQuotedIdentifier) {
		// Implicit alias (but not if it's a keyword)
		if !p.matchKeyword("JOIN", "INNER", "LEFT", "RIGHT", "FULL", "CROSS", "NATURAL", "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "INTERSECT", "EXCEPT") {
			table.Alias = p.current.Value
			p.advance()
		}
	}

	// Store lateral flag if needed (would need to extend TableReference struct)
	_ = lateral

	return table, nil
}

// parseJoinType parses join type keywords
func (p *Parser) parseJoinType() (JoinType, error) {
	if p.matchKeyword("NATURAL") {
		p.advance()
		if err := p.consumeKeyword("JOIN", "expected JOIN after NATURAL"); err != nil {
			return 0, err
		}
		return NaturalJoin, nil
	}

	if p.matchKeyword("CROSS") {
		p.advance()
		if err := p.consumeKeyword("JOIN", "expected JOIN after CROSS"); err != nil {
			return 0, err
		}
		return CrossJoin, nil
	}

	joinType := InnerJoin

	if p.matchKeyword("INNER") {
		p.advance()
		joinType = InnerJoin
	} else if p.matchKeyword("LEFT") {
		p.advance()
		if p.matchKeyword("OUTER") {
			p.advance()
			joinType = LeftOuterJoin
		} else {
			joinType = LeftJoin
		}
	} else if p.matchKeyword("RIGHT") {
		p.advance()
		if p.matchKeyword("OUTER") {
			p.advance()
			joinType = RightOuterJoin
		} else {
			joinType = RightJoin
		}
	} else if p.matchKeyword("FULL") {
		p.advance()
		if p.matchKeyword("OUTER") {
			p.advance()
			joinType = FullOuterJoin
		} else {
			joinType = FullJoin
		}
	}

	if err := p.consumeKeyword("JOIN", "expected JOIN"); err != nil {
		return 0, err
	}

	return joinType, nil
}

// parseExpression parses a SQL expression
func (p *Parser) parseExpression() (Expression, error) {
	return p.parseOrExpression()
}

// parseOrExpression parses OR expressions
func (p *Parser) parseOrExpression() (Expression, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("OR") {
		p.advance()
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpression{
			Left:     left,
			Operator: OpOr,
			Right:    right,
		}
	}

	return left, nil
}

// parseAndExpression parses AND expressions
func (p *Parser) parseAndExpression() (Expression, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("AND") {
		p.advance()
		right, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpression{
			Left:     left,
			Operator: OpAnd,
			Right:    right,
		}
	}

	return left, nil
}

// parseNotExpression parses NOT and EXISTS expressions
func (p *Parser) parseNotExpression() (Expression, error) {
	if p.matchKeyword("NOT") {
		p.advance()
		expr, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return &UnaryExpression{
			Operator: UnaryOpNot,
			Operand:  expr,
		}, nil
	}

	if p.matchKeyword("EXISTS") {
		p.advance()
		expr, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpression{
			Left:     nil, // EXISTS doesn't have a left operand
			Operator: OpExists,
			Right:    expr,
		}, nil
	}

	return p.parseComparisonExpression()
}

// parseComparisonExpression parses comparison expressions
func (p *Parser) parseComparisonExpression() (Expression, error) {
	left, err := p.parseArithmeticExpression()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOperator
		var found bool

		switch {
		case p.match(TokenEqual):
			op, found = OpEqual, true
		case p.match(TokenNotEqual):
			op, found = OpNotEqual, true
		case p.match(TokenLess):
			op, found = OpLess, true
		case p.match(TokenLessEqual):
			op, found = OpLessEqual, true
		case p.match(TokenGreater):
			op, found = OpGreater, true
		case p.match(TokenGreaterEqual):
			op, found = OpGreaterEqual, true
		case p.match(TokenBitwiseNot):
			op, found = OpRegexMatch, true
		case p.matchKeyword("LIKE"):
			op, found = OpLike, true
		case p.matchKeyword("ILIKE"):
			op, found = OpILike, true
		case p.matchKeyword("IN"):
			op, found = OpIn, true
		case p.matchKeyword("BETWEEN"):
			// Handle BETWEEN specially
			p.advance() // consume BETWEEN

			lower, err := p.parseArithmeticExpression()
			if err != nil {
				return nil, err
			}

			if p.match(TokenAnd) || p.matchKeyword("AND") {
				p.advance()
			} else {
				return nil, p.error("expected AND in BETWEEN expression")
			}

			upper, err := p.parseArithmeticExpression()
			if err != nil {
				return nil, err
			}

			// Create BETWEEN as two comparisons: left >= lower AND left <= upper
			lowerComp := &BinaryExpression{
				Left:     left,
				Operator: OpGreaterEqual,
				Right:    lower,
			}
			upperComp := &BinaryExpression{
				Left:     left,
				Operator: OpLessEqual,
				Right:    upper,
			}

			return &BinaryExpression{
				Left:     lowerComp,
				Operator: OpAnd,
				Right:    upperComp,
			}, nil
		case p.matchKeyword("IS"):
			p.advance() // consume IS

			if p.matchKeyword("NOT") || p.match(TokenNot) {
				p.advance() // consume NOT
				if p.matchKeyword("NULL") || p.match(TokenNull) {
					p.advance() // consume NULL
					return &BinaryExpression{
						Left:     left,
						Operator: OpNotEqual,
						Right:    NewNullLiteral(),
					}, nil
				} else {
					return nil, p.error("expected NULL after IS NOT")
				}
			} else if p.matchKeyword("NULL") || p.match(TokenNull) {
				p.advance() // consume NULL
				return &BinaryExpression{
					Left:     left,
					Operator: OpEqual,
					Right:    NewNullLiteral(),
				}, nil
			} else {
				return nil, p.error("expected NULL or NOT NULL after IS")
			}
		case p.matchKeyword("NOT"):
			// Handle NOT LIKE, NOT IN, NOT BETWEEN, etc.
			next := p.peek()
			if next.Type == TokenKeyword {
				switch strings.ToUpper(next.Value) {
				case "LIKE":
					p.advance() // consume NOT
					p.advance() // consume LIKE
					op, found = OpNotLike, true
				case "ILIKE":
					p.advance() // consume NOT
					p.advance() // consume ILIKE
					op, found = OpNotILike, true
				case "IN":
					p.advance() // consume NOT
					p.advance() // consume IN
					op, found = OpNotIn, true
				case "BETWEEN":
					p.advance() // consume NOT
					p.advance() // consume BETWEEN

					lower, err := p.parseArithmeticExpression()
					if err != nil {
						return nil, err
					}

					if p.match(TokenAnd) || p.matchKeyword("AND") {
						p.advance()
					} else {
						return nil, p.error("expected AND in BETWEEN expression")
					}

					upper, err := p.parseArithmeticExpression()
					if err != nil {
						return nil, err
					}

					// Create NOT BETWEEN as: left < lower OR left > upper
					lowerComp := &BinaryExpression{
						Left:     left,
						Operator: OpLess,
						Right:    lower,
					}
					upperComp := &BinaryExpression{
						Left:     left,
						Operator: OpGreater,
						Right:    upper,
					}

					return &BinaryExpression{
						Left:     lowerComp,
						Operator: OpOr,
						Right:    upperComp,
					}, nil
				}
			}
		}

		if !found {
			break
		}

		if found {
			p.advance()
		}

		right, err := p.parseArithmeticExpression()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left, nil
}

// parseArithmeticExpression parses arithmetic expressions
func (p *Parser) parseArithmeticExpression() (Expression, error) {
	left, err := p.parseTermExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TokenPlus, TokenMinus, TokenConcat) {
		var op BinaryOperator
		switch p.current.Type {
		case TokenPlus:
			op = OpPlus
		case TokenMinus:
			op = OpMinus
		case TokenConcat:
			op = OpConcat
		}

		p.advance()
		right, err := p.parseTermExpression()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left, nil
}

// parseTermExpression parses term expressions (*, /, %)
func (p *Parser) parseTermExpression() (Expression, error) {
	left, err := p.parseUnaryExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TokenMultiply, TokenDivide, TokenModulo) {
		var op BinaryOperator
		switch p.current.Type {
		case TokenMultiply:
			op = OpMultiply
		case TokenDivide:
			op = OpDivide
		case TokenModulo:
			op = OpModulo
		}

		p.advance()
		right, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left, nil
}

// parseUnaryExpression parses unary expressions
func (p *Parser) parseUnaryExpression() (Expression, error) {
	if p.match(TokenMinus, TokenPlus, TokenBitwiseNot) {
		var op UnaryOperator
		switch p.current.Type {
		case TokenMinus:
			op = UnaryOpMinus
		case TokenPlus:
			op = UnaryOpPlus
		case TokenBitwiseNot:
			op = UnaryOpBitwiseNot
		}

		p.advance()
		expr, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}

		return &UnaryExpression{
			Operator: op,
			Operand:  expr,
		}, nil
	}

	return p.parsePrimaryExpression()
}

// parsePrimaryExpression parses primary expressions
func (p *Parser) parsePrimaryExpression() (Expression, error) {
	switch p.current.Type {
	case TokenString:
		value := p.current.Value
		p.advance()
		return NewStringLiteral(value), nil

	case TokenInteger:
		value, err := strconv.ParseInt(p.current.Value, 10, 64)
		if err != nil {
			return nil, p.error("invalid integer literal")
		}
		p.advance()
		return NewIntegerLiteral(value), nil

	case TokenFloat:
		value, err := strconv.ParseFloat(p.current.Value, 64)
		if err != nil {
			return nil, p.error("invalid float literal")
		}
		p.advance()
		return NewFloatLiteral(value), nil

	case TokenBoolean:
		value := strings.ToUpper(p.current.Value) == "TRUE"
		p.advance()
		return NewBooleanLiteral(value), nil

	case TokenNull:
		p.advance()
		return NewNullLiteral(), nil

	case TokenIdentifier, TokenQuotedIdentifier:
		return p.parseIdentifierOrFunction()

	case TokenMultiply:
		// Handle * as a special identifier (for SELECT *)
		p.advance()
		return NewIdentifier("*"), nil

	case TokenLeftParen:
		p.advance()

		// Check if this is a subquery
		if p.matchKeyword("SELECT", "WITH") {
			subquery, err := p.parseSelectStatement()
			if err != nil {
				return nil, err
			}
			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
			return &Subquery{Query: subquery}, nil
		}

		// Regular parenthesized expression
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenExists:
		p.advance()
		expr, err := p.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpression{
			Left:     nil,
			Operator: OpExists,
			Right:    expr,
		}, nil

	case TokenLeftBracket:
		// Array literal
		p.advance()
		var elements []Expression

		if !p.match(TokenRightBracket) {
			for {
				element, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				elements = append(elements, element)

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}
		}

		if err := p.consume(TokenRightBracket, "expected ']'"); err != nil {
			return nil, err
		}

		return &LiteralExpression{
			Value: elements,
			Type:  LiteralArray,
		}, nil

	case TokenKeyword:
		if p.matchKeyword("CASE") {
			return p.parseCaseExpression()
		} else if p.matchKeyword("CAST") {
			return p.parseCastExpression()
		} else if p.matchKeyword("EXTRACT") {
			return p.parseExtractExpression()
		} else if p.matchKeyword("ANY", "ALL", "SOME") {
			// Handle ANY/ALL/SOME as function calls
			return p.parseIdentifierOrFunction()
		}
		fallthrough

	default:
		return nil, p.error(fmt.Sprintf("unexpected token: %s", p.current.Value))
	}
}

// parseIdentifierOrFunction parses identifiers or function calls
func (p *Parser) parseIdentifierOrFunction() (Expression, error) {
	name := p.current.Value
	p.advance()

	// Check for qualified identifier (schema.table.column or table.column)
	var schema, table string
	if p.match(TokenDot) {
		p.advance()
		if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
			return nil, p.error("expected identifier after '.'")
		}

		table = name
		name = p.current.Value
		p.advance()

		if p.match(TokenDot) {
			p.advance()
			if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
				return nil, p.error("expected identifier after '.'")
			}

			schema = table
			table = name
			name = p.current.Value
			p.advance()
		}
	}

	// Check if this is a function call
	if p.match(TokenLeftParen) {
		p.advance()

		// Handle special functions like CAST and EXTRACT
		if strings.ToUpper(name) == "CAST" {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}

			if p.matchKeyword("AS") {
				p.advance()
			} else {
				return nil, p.error("expected AS in CAST expression")
			}

			dataType, err := p.parseDataType()
			if err != nil {
				return nil, err
			}

			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}

			// Create a function call to represent CAST
			return &FunctionCall{
				Name:      "CAST",
				Arguments: []Expression{expr, &LiteralExpression{Value: dataType.Name, Type: LiteralString}},
			}, nil
		} else if strings.ToUpper(name) == "EXTRACT" {
			// Parse field (YEAR, MONTH, DAY, etc.)
			if !p.match(TokenIdentifier, TokenKeyword) {
				return nil, p.error("expected field name")
			}
			field := p.current.Value
			p.advance()

			if p.matchKeyword("FROM") {
				p.advance()
			} else {
				return nil, p.error("expected FROM in EXTRACT expression")
			}

			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}

			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}

			// Create a function call to represent EXTRACT
			return &FunctionCall{
				Name:      "EXTRACT",
				Arguments: []Expression{&LiteralExpression{Value: field, Type: LiteralString}, expr},
			}, nil
		}

		function := &FunctionCall{Name: name}

		// Parse DISTINCT if present
		if p.matchKeyword("DISTINCT") {
			function.Distinct = true
			p.advance()
		}

		// Parse arguments
		if !p.match(TokenRightParen) {
			for {
				arg, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				function.Arguments = append(function.Arguments, arg)

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}
		}

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}

		// Parse FILTER clause if present
		if p.matchKeyword("FILTER") {
			p.advance()
			if err := p.consume(TokenLeftParen, "expected '(' after FILTER"); err != nil {
				return nil, err
			}
			if err := p.consumeKeyword("WHERE", "expected WHERE after FILTER ("); err != nil {
				return nil, err
			}

			filter, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			function.Filter = filter

			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
		}

		// Parse OVER clause if present (window functions)
		if p.matchKeyword("OVER") {
			p.advance()
			windowSpec, err := p.parseWindowSpec()
			if err != nil {
				return nil, err
			}
			function.Over = windowSpec
		}

		return function, nil
	}

	// Regular identifier
	return &IdentifierExpression{
		Schema: schema,
		Table:  table,
		Name:   name,
	}, nil
}

// parseCaseExpression parses CASE expressions
func (p *Parser) parseCaseExpression() (Expression, error) {
	p.advance() // consume CASE

	caseExpr := &CaseExpression{}

	// Check if this is a simple CASE (CASE expr WHEN ...) or searched CASE (CASE WHEN ...)
	if !p.matchKeyword("WHEN") {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		caseExpr.Expression = expr
	}

	// Parse WHEN clauses
	for p.matchKeyword("WHEN") {
		p.advance()

		condition, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if err := p.consumeKeyword("THEN", "expected THEN after WHEN condition"); err != nil {
			return nil, err
		}

		result, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		caseExpr.WhenClauses = append(caseExpr.WhenClauses, &WhenClause{
			Condition: condition,
			Result:    result,
		})
	}

	// Parse optional ELSE clause
	if p.matchKeyword("ELSE") {
		p.advance()
		elseExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		caseExpr.ElseClause = elseExpr
	}

	if err := p.consumeKeyword("END", "expected END after CASE expression"); err != nil {
		return nil, err
	}

	return caseExpr, nil
}

// parseCastExpression parses CAST expressions
func (p *Parser) parseCastExpression() (Expression, error) {
	p.advance() // consume CAST

	if err := p.consume(TokenLeftParen, "expected '(' after CAST"); err != nil {
		return nil, err
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.matchKeyword("AS") {
		p.advance()
	} else {
		return nil, p.error("expected AS in CAST expression")
	}

	dataType, err := p.parseDataType()
	if err != nil {
		return nil, err
	}

	if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
		return nil, err
	}

	// Create a function call to represent CAST
	return &FunctionCall{
		Name:      "CAST",
		Arguments: []Expression{expr, &LiteralExpression{Value: dataType.Name, Type: LiteralString}},
	}, nil
}

// parseExtractExpression parses EXTRACT expressions
func (p *Parser) parseExtractExpression() (Expression, error) {
	p.advance() // consume EXTRACT

	if err := p.consume(TokenLeftParen, "expected '(' after EXTRACT"); err != nil {
		return nil, err
	}

	// Parse field (YEAR, MONTH, DAY, etc.)
	if !p.match(TokenIdentifier, TokenKeyword) {
		return nil, p.error("expected field name")
	}
	field := p.current.Value
	p.advance()

	if p.matchKeyword("FROM") {
		p.advance()
	} else {
		return nil, p.error("expected FROM in EXTRACT expression")
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
		return nil, err
	}

	// Create a function call to represent EXTRACT
	return &FunctionCall{
		Name:      "EXTRACT",
		Arguments: []Expression{&LiteralExpression{Value: field, Type: LiteralString}, expr},
	}, nil
}

// parseWindowSpec parses window specifications
func (p *Parser) parseWindowSpec() (*WindowSpec, error) {
	if err := p.consume(TokenLeftParen, "expected '(' after OVER"); err != nil {
		return nil, err
	}

	spec := &WindowSpec{}

	// Parse PARTITION BY clause
	if p.matchKeyword("PARTITION") {
		p.advance()
		if err := p.consumeKeyword("BY", "expected BY after PARTITION"); err != nil {
			return nil, err
		}

		partitionBy, err := p.parseExpressionList()
		if err != nil {
			return nil, err
		}
		spec.PartitionBy = partitionBy
	}

	// Parse ORDER BY clause
	if p.matchKeyword("ORDER") {
		p.advance()
		if err := p.consumeKeyword("BY", "expected BY after ORDER"); err != nil {
			return nil, err
		}

		orderBy, err := p.parseOrderByClause()
		if err != nil {
			return nil, err
		}
		spec.OrderBy = orderBy
	}

	// Parse frame clause (ROWS/RANGE)
	if p.matchKeyword("ROWS", "RANGE") {
		frame, err := p.parseWindowFrame()
		if err != nil {
			return nil, err
		}
		spec.Frame = frame
	}

	if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
		return nil, err
	}

	return spec, nil
}

// parseWindowFrame parses window frame specifications
func (p *Parser) parseWindowFrame() (*WindowFrame, error) {
	frame := &WindowFrame{}

	if p.matchKeyword("ROWS") {
		frame.Type = RowsFrame
		p.advance()
	} else if p.matchKeyword("RANGE") {
		frame.Type = RangeFrame
		p.advance()
	} else {
		return nil, p.error("expected ROWS or RANGE")
	}

	// Parse frame bounds
	if p.matchKeyword("BETWEEN") {
		p.advance()

		start, err := p.parseFrameBound()
		if err != nil {
			return nil, err
		}
		frame.Start = start

		if err := p.consumeKeyword("AND", "expected AND in frame specification"); err != nil {
			return nil, err
		}

		end, err := p.parseFrameBound()
		if err != nil {
			return nil, err
		}
		frame.End = end
	} else {
		// Single bound (implies CURRENT ROW as end)
		start, err := p.parseFrameBound()
		if err != nil {
			return nil, err
		}
		frame.Start = start
	}

	return frame, nil
}

// parseFrameBound parses frame bound specifications
func (p *Parser) parseFrameBound() (*FrameBound, error) {
	bound := &FrameBound{}

	if p.matchKeyword("UNBOUNDED") {
		p.advance()
		if p.matchKeyword("PRECEDING") {
			bound.Type = UnboundedPreceding
			bound.Preceding = true
			p.advance()
		} else if p.matchKeyword("FOLLOWING") {
			bound.Type = UnboundedFollowing
			bound.Following = true
			p.advance()
		} else {
			return nil, p.error("expected PRECEDING or FOLLOWING after UNBOUNDED")
		}
	} else if p.matchKeyword("CURRENT") {
		p.advance()
		if err := p.consumeKeyword("ROW", "expected ROW after CURRENT"); err != nil {
			return nil, err
		}
		bound.Type = CurrentRow
	} else {
		// Expression PRECEDING/FOLLOWING
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		bound.Value = expr

		if p.matchKeyword("PRECEDING") {
			bound.Type = ValuePreceding
			bound.Preceding = true
			p.advance()
		} else if p.matchKeyword("FOLLOWING") {
			bound.Type = ValueFollowing
			bound.Following = true
			p.advance()
		} else {
			return nil, p.error("expected PRECEDING or FOLLOWING after expression")
		}
	}

	return bound, nil
}

// parseExpressionList parses a comma-separated list of expressions
func (p *Parser) parseExpressionList() ([]Expression, error) {
	var expressions []Expression

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, expr)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return expressions, nil
}

// parseOrderByClause parses ORDER BY clauses
func (p *Parser) parseOrderByClause() ([]OrderByClause, error) {
	var clauses []OrderByClause

	for {
		clause := OrderByClause{}

		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		clause.Expression = expr

		// Parse direction
		if p.matchKeyword("ASC") {
			clause.Direction = Ascending
			p.advance()
		} else if p.matchKeyword("DESC") {
			clause.Direction = Descending
			p.advance()
		} else {
			clause.Direction = Ascending // default
		}

		// Parse NULLS FIRST/LAST
		if p.matchKeyword("NULLS") {
			p.advance()
			if p.matchKeyword("FIRST") {
				clause.NullsFirst = true
				p.advance()
			} else if p.matchKeyword("LAST") {
				clause.NullsFirst = false
				p.advance()
			} else {
				return nil, p.error("expected FIRST or LAST after NULLS")
			}
		}

		clauses = append(clauses, clause)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return clauses, nil
}

// parseWindowDefinitions parses WINDOW clause definitions
func (p *Parser) parseWindowDefinitions() ([]*WindowDefinition, error) {
	p.advance() // consume WINDOW

	var definitions []*WindowDefinition

	for {
		def := &WindowDefinition{}

		if !p.match(TokenIdentifier) {
			return nil, p.error("expected window name")
		}
		def.Name = p.current.Value
		p.advance()

		if err := p.consumeKeyword("AS", "expected AS after window name"); err != nil {
			return nil, err
		}

		spec, err := p.parseWindowSpec()
		if err != nil {
			return nil, err
		}
		def.Spec = spec

		definitions = append(definitions, def)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return definitions, nil
}

// Placeholder implementations for other statement types
// These would be implemented similarly to parseSelectStatement

func (p *Parser) parseInsertStatement() (*InsertStatement, error) {
	stmt := &InsertStatement{}

	// Parse INSERT
	if err := p.consumeKeyword("INSERT", "expected INSERT"); err != nil {
		return nil, err
	}

	// Parse INTO
	if err := p.consumeKeyword("INTO", "expected INTO"); err != nil {
		return nil, err
	}

	// Parse table reference
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.Table = table

	// Parse optional column list
	if p.match(TokenLeftParen) {
		p.advance()
		for {
			if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
				return nil, p.error("expected column name")
			}
			stmt.Columns = append(stmt.Columns, p.current.Value)
			p.advance()

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}
		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	}

	// Parse VALUES or SELECT
	if p.matchKeyword("VALUES") {
		p.advance()
		for {
			if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
				return nil, err
			}

			var values []Expression
			for {
				value, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				values = append(values, value)

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}

			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}

			stmt.Values = append(stmt.Values, values)

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}
	} else if p.matchKeyword("SELECT") {
		selectStmt, err := p.parseSelectStatement()
		if err != nil {
			return nil, err
		}
		stmt.Select = selectStmt
	} else {
		return nil, p.error("expected VALUES or SELECT")
	}

	// Parse optional ON CONFLICT clause
	if p.matchKeyword("ON") {
		next := p.peek()
		if next.Type == TokenKeyword && strings.ToUpper(next.Value) == "CONFLICT" {
			p.advance() // consume ON
			p.advance() // consume CONFLICT

			onConflict := &OnConflictClause{}

			// Parse optional conflict target
			if p.match(TokenLeftParen) {
				p.advance()
				for {
					if !p.match(TokenIdentifier) {
						return nil, p.error("expected column name")
					}
					onConflict.Columns = append(onConflict.Columns, p.current.Value)
					p.advance()

					if p.match(TokenComma) {
						p.advance()
					} else {
						break
					}
				}
				if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
					return nil, err
				}
			}

			// Parse DO NOTHING or DO UPDATE
			if err := p.consumeKeyword("DO", "expected DO"); err != nil {
				return nil, err
			}

			if p.matchKeyword("NOTHING") {
				p.advance()
				onConflict.Action = DoNothing
			} else if p.matchKeyword("UPDATE") {
				p.advance()
				onConflict.Action = DoUpdate

				// Parse SET clause
				if err := p.consumeKeyword("SET", "expected SET"); err != nil {
					return nil, err
				}

				for {
					setClause := SetClause{}

					if !p.match(TokenIdentifier) {
						return nil, p.error("expected column name")
					}
					setClause.Column = p.current.Value
					p.advance()

					if err := p.consume(TokenEqual, "expected '='"); err != nil {
						return nil, err
					}

					value, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					setClause.Value = value

					onConflict.Set = append(onConflict.Set, setClause)

					if p.match(TokenComma) {
						p.advance()
					} else {
						break
					}
				}

				// Parse optional WHERE clause
				if p.matchKeyword("WHERE") {
					p.advance()
					where, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					onConflict.Where = where
				}
			} else {
				return nil, p.error("expected NOTHING or UPDATE after DO")
			}

			stmt.OnConflict = onConflict
		}
	}

	return stmt, nil
}

func (p *Parser) parseUpdateStatement() (*UpdateStatement, error) {
	stmt := &UpdateStatement{}

	// Parse UPDATE
	if err := p.consumeKeyword("UPDATE", "expected UPDATE"); err != nil {
		return nil, err
	}

	// Parse table reference
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.Table = table

	// Parse SET clause
	if err := p.consumeKeyword("SET", "expected SET"); err != nil {
		return nil, err
	}

	for {
		setClause := SetClause{}

		if !p.match(TokenIdentifier) {
			return nil, p.error("expected column name")
		}
		setClause.Column = p.current.Value
		p.advance()

		if err := p.consume(TokenEqual, "expected '='"); err != nil {
			return nil, err
		}

		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		setClause.Value = value

		stmt.Set = append(stmt.Set, setClause)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	// Parse optional FROM clause
	if p.matchKeyword("FROM") {
		p.advance()
		from, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}
		stmt.From = from
	}

	// Parse optional WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

func (p *Parser) parseDeleteStatement() (*DeleteStatement, error) {
	stmt := &DeleteStatement{}

	// Parse DELETE
	if err := p.consumeKeyword("DELETE", "expected DELETE"); err != nil {
		return nil, err
	}

	// Parse FROM
	if err := p.consumeKeyword("FROM", "expected FROM"); err != nil {
		return nil, err
	}

	// Parse table reference
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.From = table

	// Parse optional USING clause
	if p.matchKeyword("USING") {
		p.advance()
		for {
			table, err := p.parseTableReference()
			if err != nil {
				return nil, err
			}
			stmt.Using = append(stmt.Using, *table)

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}
	}

	// Parse optional WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

func (p *Parser) parseCreateStatement() (Statement, error) {
	p.advance() // consume CREATE

	if p.matchKeyword("TEMPORARY", "TEMP") {
		p.advance() // consume TEMPORARY/TEMP
		if p.matchKeyword("TABLE") {
			return p.parseCreateTableStatement()
		}
		return nil, p.error("expected TABLE after TEMPORARY")
	} else if p.matchKeyword("UNIQUE") {
		p.advance() // consume UNIQUE
		if p.matchKeyword("INDEX") {
			return p.parseCreateIndexStatement()
		}
		return nil, p.error("expected INDEX after UNIQUE")
	} else if p.matchKeyword("TABLE") {
		return p.parseCreateTableStatement()
	} else if p.matchKeyword("INDEX") {
		return p.parseCreateIndexStatement()
	}

	return nil, p.error("unsupported CREATE statement")
}

func (p *Parser) parseCreateTableStatement() (*CreateTableStatement, error) {
	stmt := &CreateTableStatement{}

	// Check if TEMPORARY was already consumed in parseCreateStatement
	if !p.matchKeyword("TABLE") {
		// TEMPORARY was already consumed, so this is a temporary table
		stmt.Temporary = true
	}

	if p.matchKeyword("TABLE") {
		p.advance() // consume TABLE
	}

	// Parse optional IF NOT EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if p.match(TokenNot) || p.matchKeyword("NOT") {
			p.advance()
		} else {
			return nil, p.error("expected NOT after IF")
		}
		if p.match(TokenExists) || p.matchKeyword("EXISTS") {
			p.advance()
		} else {
			return nil, p.error("expected EXISTS after NOT")
		}
		stmt.IfNotExists = true
	}

	// Parse table name
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.Table = table

	// Parse column definitions and constraints
	if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
		return nil, err
	}

	for {
		// Check if this is a table constraint
		if p.matchKeyword("CONSTRAINT", "PRIMARY", "UNIQUE", "FOREIGN", "CHECK") {
			constraint, err := p.parseTableConstraint()
			if err != nil {
				return nil, err
			}
			stmt.Constraints = append(stmt.Constraints, constraint)
		} else {
			// Parse column definition
			column, err := p.parseColumnDefinition()
			if err != nil {
				return nil, err
			}
			stmt.Columns = append(stmt.Columns, column)
		}

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
		return nil, err
	}

	return stmt, nil
}

func (p *Parser) parseColumnDefinition() (*ColumnDefinition, error) {
	column := &ColumnDefinition{}

	// Parse column name
	if !p.match(TokenIdentifier, TokenQuotedIdentifier) {
		return nil, p.error("expected column name")
	}
	column.Name = p.current.Value
	p.advance()

	// Parse data type
	dataType, err := p.parseDataType()
	if err != nil {
		return nil, err
	}
	column.Type = dataType

	// Parse column constraints
	for p.matchKeyword("CONSTRAINT", "NOT", "NULL", "PRIMARY", "UNIQUE", "REFERENCES", "CHECK", "DEFAULT", "GENERATED") || p.match(TokenNot) {
		constraint, err := p.parseColumnConstraint()
		if err != nil {
			return nil, err
		}
		if constraint != nil {
			column.Constraints = append(column.Constraints, constraint)
		}
	}

	return column, nil
}

func (p *Parser) parseDataType() (*DataType, error) {
	dataType := &DataType{}

	if !p.match(TokenIdentifier, TokenKeyword) {
		return nil, p.error("expected data type")
	}

	dataType.Name = strings.ToUpper(p.current.Value)
	p.advance()

	// Parse optional length/precision
	if p.match(TokenLeftParen) {
		p.advance()

		// Parse first number (length or precision)
		if !p.match(TokenInteger) {
			return nil, p.error("expected integer")
		}
		length, err := strconv.Atoi(p.current.Value)
		if err != nil {
			return nil, p.error("invalid integer")
		}
		dataType.Length = length
		dataType.Precision = length
		p.advance()

		// Parse optional second number (scale)
		if p.match(TokenComma) {
			p.advance()
			if !p.match(TokenInteger) {
				return nil, p.error("expected integer")
			}
			scale, err := strconv.Atoi(p.current.Value)
			if err != nil {
				return nil, p.error("invalid integer")
			}
			dataType.Scale = scale
			p.advance()
		}

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	}

	// Parse optional ARRAY
	if p.match(TokenLeftBracket) {
		p.advance()
		if err := p.consume(TokenRightBracket, "expected ']'"); err != nil {
			return nil, err
		}
		dataType.Array = true
	}

	return dataType, nil
}

func (p *Parser) parseColumnConstraint() (*ColumnConstraint, error) {
	constraint := &ColumnConstraint{}

	// Parse optional constraint name
	if p.matchKeyword("CONSTRAINT") {
		p.advance()
		if !p.match(TokenIdentifier) {
			return nil, p.error("expected constraint name")
		}
		constraint.Name = p.current.Value
		p.advance()
	}

	// Parse constraint type
	if p.matchKeyword("NOT") || p.match(TokenNot) {
		p.advance()
		if p.match(TokenNull) || p.matchKeyword("NULL") {
			p.advance()
		} else {
			return nil, p.error("expected NULL after NOT")
		}
		constraint.Type = NotNullConstraint
		constraint.NotNull = true
	} else if p.matchKeyword("NULL") {
		p.advance()
		// NULL constraint (explicitly nullable) - return nil to skip adding this constraint
		return nil, nil
	} else if p.matchKeyword("PRIMARY") {
		p.advance()
		if err := p.consumeKeyword("KEY", "expected KEY after PRIMARY"); err != nil {
			return nil, err
		}
		constraint.Type = PrimaryKeyConstraint
		constraint.PrimaryKey = true
	} else if p.matchKeyword("UNIQUE") {
		p.advance()
		constraint.Type = UniqueConstraint
		constraint.Unique = true
	} else if p.matchKeyword("REFERENCES") {
		p.advance()
		constraint.Type = ForeignKeyConstraint

		// Parse referenced table
		if !p.match(TokenIdentifier) {
			return nil, p.error("expected table name")
		}
		refTable := &TableReference{Name: p.current.Value}
		p.advance()

		// Parse optional column list
		var refColumns []string
		if p.match(TokenLeftParen) {
			p.advance()
			for {
				if !p.match(TokenIdentifier) {
					return nil, p.error("expected column name")
				}
				refColumns = append(refColumns, p.current.Value)
				p.advance()

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}
			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
		}

		constraint.References = &ForeignKeyReference{
			Table:   refTable,
			Columns: refColumns,
		}

		// Parse optional ON DELETE/UPDATE actions
		for p.matchKeyword("ON") {
			p.advance()
			if p.matchKeyword("DELETE") {
				p.advance()
				action, err := p.parseReferentialAction()
				if err != nil {
					return nil, err
				}
				constraint.References.OnDelete = action
			} else if p.matchKeyword("UPDATE") {
				p.advance()
				action, err := p.parseReferentialAction()
				if err != nil {
					return nil, err
				}
				constraint.References.OnUpdate = action
			} else {
				return nil, p.error("expected DELETE or UPDATE after ON")
			}
		}
	} else if p.matchKeyword("CHECK") {
		p.advance()
		constraint.Type = CheckConstraint

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		checkExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		constraint.Check = checkExpr

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	} else if p.matchKeyword("DEFAULT") {
		p.advance()
		constraint.Type = DefaultConstraint

		defaultExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		constraint.Default = defaultExpr
	} else if p.matchKeyword("GENERATED") {
		p.advance()
		if err := p.consumeKeyword("ALWAYS", "expected ALWAYS after GENERATED"); err != nil {
			return nil, err
		}
		if err := p.consumeKeyword("AS", "expected AS after ALWAYS"); err != nil {
			return nil, err
		}

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		genExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		constraint.Default = genExpr

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}

		// This is a generated column, we could add a specific constraint type for this
		constraint.Type = DefaultConstraint
	} else {
		return nil, p.error("expected constraint type")
	}

	return constraint, nil
}

func (p *Parser) parseReferentialAction() (ReferentialAction, error) {
	if p.matchKeyword("NO") {
		p.advance()
		if err := p.consumeKeyword("ACTION", "expected ACTION after NO"); err != nil {
			return 0, err
		}
		return NoAction, nil
	} else if p.matchKeyword("RESTRICT") {
		p.advance()
		return Restrict, nil
	} else if p.matchKeyword("CASCADE") {
		p.advance()
		return Cascade, nil
	} else if p.matchKeyword("SET") {
		p.advance()
		if p.matchKeyword("NULL") {
			p.advance()
			return SetNull, nil
		} else if p.matchKeyword("DEFAULT") {
			p.advance()
			return SetDefault, nil
		} else {
			return 0, p.error("expected NULL or DEFAULT after SET")
		}
	} else {
		return 0, p.error("expected referential action")
	}
}

func (p *Parser) parseTableConstraint() (*TableConstraint, error) {
	constraint := &TableConstraint{}

	// Parse optional constraint name
	if p.matchKeyword("CONSTRAINT") {
		p.advance()
		if !p.match(TokenIdentifier) {
			return nil, p.error("expected constraint name")
		}
		constraint.Name = p.current.Value
		p.advance()
	}

	// Parse constraint type
	if p.matchKeyword("PRIMARY") {
		p.advance()
		if err := p.consumeKeyword("KEY", "expected KEY after PRIMARY"); err != nil {
			return nil, err
		}
		constraint.Type = PrimaryKeyConstraint

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		for {
			if !p.match(TokenIdentifier) {
				return nil, p.error("expected column name")
			}
			constraint.Columns = append(constraint.Columns, p.current.Value)
			p.advance()

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	} else if p.matchKeyword("UNIQUE") {
		p.advance()
		constraint.Type = UniqueConstraint

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		for {
			if !p.match(TokenIdentifier) {
				return nil, p.error("expected column name")
			}
			constraint.Columns = append(constraint.Columns, p.current.Value)
			p.advance()

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	} else if p.matchKeyword("FOREIGN") {
		p.advance()
		if err := p.consumeKeyword("KEY", "expected KEY after FOREIGN"); err != nil {
			return nil, err
		}
		constraint.Type = ForeignKeyConstraint

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		for {
			if !p.match(TokenIdentifier) {
				return nil, p.error("expected column name")
			}
			constraint.Columns = append(constraint.Columns, p.current.Value)
			p.advance()

			if p.match(TokenComma) {
				p.advance()
			} else {
				break
			}
		}

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}

		if err := p.consumeKeyword("REFERENCES", "expected REFERENCES"); err != nil {
			return nil, err
		}

		// Parse referenced table
		if !p.match(TokenIdentifier) {
			return nil, p.error("expected table name")
		}
		refTable := &TableReference{Name: p.current.Value}
		p.advance()

		// Parse referenced columns
		var refColumns []string
		if p.match(TokenLeftParen) {
			p.advance()
			for {
				if !p.match(TokenIdentifier) {
					return nil, p.error("expected column name")
				}
				refColumns = append(refColumns, p.current.Value)
				p.advance()

				if p.match(TokenComma) {
					p.advance()
				} else {
					break
				}
			}
			if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
				return nil, err
			}
		}

		constraint.References = &ForeignKeyReference{
			Table:   refTable,
			Columns: refColumns,
		}

		// Parse optional ON DELETE/UPDATE actions
		for p.matchKeyword("ON") {
			p.advance()
			if p.matchKeyword("DELETE") {
				p.advance()
				action, err := p.parseReferentialAction()
				if err != nil {
					return nil, err
				}
				constraint.References.OnDelete = action
			} else if p.matchKeyword("UPDATE") {
				p.advance()
				action, err := p.parseReferentialAction()
				if err != nil {
					return nil, err
				}
				constraint.References.OnUpdate = action
			} else {
				return nil, p.error("expected DELETE or UPDATE after ON")
			}
		}
	} else if p.matchKeyword("CHECK") {
		p.advance()
		constraint.Type = CheckConstraint

		if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
			return nil, err
		}

		checkExpr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		constraint.Check = checkExpr

		if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
			return nil, err
		}
	} else {
		return nil, p.error("expected constraint type")
	}

	return constraint, nil
}

func (p *Parser) parseCreateIndexStatement() (*CreateIndexStatement, error) {
	stmt := &CreateIndexStatement{}

	// Check if UNIQUE was already consumed in parseCreateStatement
	if !p.matchKeyword("INDEX") {
		// UNIQUE was already consumed, so this is a unique index
		stmt.Unique = true
	}

	if p.matchKeyword("INDEX") {
		p.advance() // consume INDEX
	}

	// Parse optional IF NOT EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if p.match(TokenNot) || p.matchKeyword("NOT") {
			p.advance()
		} else {
			return nil, p.error("expected NOT after IF")
		}
		if p.match(TokenExists) || p.matchKeyword("EXISTS") {
			p.advance()
		} else {
			return nil, p.error("expected EXISTS after NOT")
		}
		stmt.IfNotExists = true
	}

	// Parse index name
	if !p.match(TokenIdentifier) {
		return nil, p.error("expected index name")
	}
	stmt.Name = p.current.Value
	p.advance()

	// Parse ON
	if err := p.consumeKeyword("ON", "expected ON"); err != nil {
		return nil, err
	}

	// Parse table name
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.Table = table

	// Parse column list
	if err := p.consume(TokenLeftParen, "expected '('"); err != nil {
		return nil, err
	}

	for {
		column := IndexColumn{}

		// Parse column name or expression
		if p.match(TokenIdentifier) {
			// Check if this is a function call (identifier followed by '(')
			if p.peek().Type == TokenLeftParen {
				// This is a function call, parse as expression
				expr, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				column.Expression = expr
			} else {
				// Simple column name
				column.Name = p.current.Value
				p.advance()
			}
		} else {
			// Parse expression
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			column.Expression = expr
		}

		// Parse optional direction
		if p.matchKeyword("ASC") {
			column.Direction = Ascending
			p.advance()
		} else if p.matchKeyword("DESC") {
			column.Direction = Descending
			p.advance()
		} else {
			column.Direction = Ascending // default
		}

		stmt.Columns = append(stmt.Columns, column)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	if err := p.consume(TokenRightParen, "expected ')'"); err != nil {
		return nil, err
	}

	// Parse optional WHERE clause
	if p.matchKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

func (p *Parser) parseDropStatement() (Statement, error) {
	p.advance() // consume DROP

	if p.matchKeyword("TABLE") {
		return p.parseDropTableStatement()
	} else if p.matchKeyword("INDEX") {
		return p.parseDropIndexStatement()
	}

	return nil, p.error("unsupported DROP statement")
}

func (p *Parser) parseDropTableStatement() (*DropTableStatement, error) {
	stmt := &DropTableStatement{}

	p.advance() // consume TABLE

	// Parse optional IF EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if p.match(TokenExists) || p.matchKeyword("EXISTS") {
			p.advance()
		} else {
			return nil, p.error("expected EXISTS after IF")
		}
		stmt.IfExists = true
	}

	// Parse table names
	for {
		table, err := p.parseTableReference()
		if err != nil {
			return nil, err
		}
		stmt.Tables = append(stmt.Tables, table)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	// Parse optional CASCADE/RESTRICT
	if p.matchKeyword("CASCADE") {
		stmt.Cascade = true
		p.advance()
	} else if p.matchKeyword("RESTRICT") {
		p.advance()
	}

	return stmt, nil
}

func (p *Parser) parseDropIndexStatement() (*DropIndexStatement, error) {
	stmt := &DropIndexStatement{}

	p.advance() // consume INDEX

	// Parse optional IF EXISTS
	if p.matchKeyword("IF") {
		p.advance()
		if p.match(TokenExists) || p.matchKeyword("EXISTS") {
			p.advance()
		} else {
			return nil, p.error("expected EXISTS after IF")
		}
		stmt.IfExists = true
	}

	// Parse index name
	if !p.match(TokenIdentifier) {
		return nil, p.error("expected index name")
	}
	stmt.Name = p.current.Value
	p.advance()

	// Parse optional CASCADE/RESTRICT
	if p.matchKeyword("CASCADE") {
		stmt.Cascade = true
		p.advance()
	} else if p.matchKeyword("RESTRICT") {
		p.advance()
	}

	return stmt, nil
}

func (p *Parser) parseAlterStatement() (Statement, error) {
	p.advance() // consume ALTER

	if p.matchKeyword("TABLE") {
		return p.parseAlterTableStatement()
	}

	return nil, p.error("unsupported ALTER statement")
}

func (p *Parser) parseAlterTableStatement() (*AlterTableStatement, error) {
	stmt := &AlterTableStatement{}

	p.advance() // consume TABLE

	// Parse table name
	table, err := p.parseTableReference()
	if err != nil {
		return nil, err
	}
	stmt.Table = table

	// Parse alter actions
	for {
		action, err := p.parseAlterTableAction()
		if err != nil {
			return nil, err
		}
		stmt.Actions = append(stmt.Actions, action)

		if p.match(TokenComma) {
			p.advance()
		} else {
			break
		}
	}

	return stmt, nil
}

func (p *Parser) parseAlterTableAction() (AlterTableAction, error) {
	if p.matchKeyword("ADD") {
		p.advance()

		if p.matchKeyword("COLUMN") {
			p.advance()
			column, err := p.parseColumnDefinition()
			if err != nil {
				return nil, err
			}
			return &AddColumnAction{Column: column}, nil
		} else if p.matchKeyword("CONSTRAINT") {
			constraint, err := p.parseTableConstraint()
			if err != nil {
				return nil, err
			}
			return &AddConstraintAction{Constraint: constraint}, nil
		} else {
			// ADD without COLUMN keyword
			column, err := p.parseColumnDefinition()
			if err != nil {
				return nil, err
			}
			return &AddColumnAction{Column: column}, nil
		}
	} else if p.matchKeyword("DROP") {
		p.advance()

		if p.matchKeyword("COLUMN") {
			p.advance()
			if !p.match(TokenIdentifier) {
				return nil, p.error("expected column name")
			}
			columnName := p.current.Value
			p.advance()

			cascade := false
			if p.matchKeyword("CASCADE") {
				cascade = true
				p.advance()
			} else if p.matchKeyword("RESTRICT") {
				p.advance()
			}

			return &DropColumnAction{Column: columnName, Cascade: cascade}, nil
		} else if p.matchKeyword("CONSTRAINT") {
			p.advance()
			if !p.match(TokenIdentifier) {
				return nil, p.error("expected constraint name")
			}
			constraintName := p.current.Value
			p.advance()

			cascade := false
			if p.matchKeyword("CASCADE") {
				cascade = true
				p.advance()
			} else if p.matchKeyword("RESTRICT") {
				p.advance()
			}

			return &DropConstraintAction{Name: constraintName, Cascade: cascade}, nil
		} else {
			return nil, p.error("expected COLUMN or CONSTRAINT after DROP")
		}
	} else if p.matchKeyword("ALTER") {
		p.advance()

		if p.matchKeyword("COLUMN") {
			p.advance()
		}

		if !p.match(TokenIdentifier) {
			return nil, p.error("expected column name")
		}
		columnName := p.current.Value
		p.advance()

		var action ColumnAlterAction

		if p.matchKeyword("SET") {
			p.advance()
			if p.matchKeyword("DATA") {
				p.advance()
				if err := p.consumeKeyword("TYPE", "expected TYPE after DATA"); err != nil {
					return nil, err
				}
				action = SetDataType
			} else if p.matchKeyword("DEFAULT") {
				p.advance()
				action = SetColumnDefault
			} else if p.matchKeyword("NOT") || p.match(TokenNot) {
				p.advance()
				if p.match(TokenNull) || p.matchKeyword("NULL") {
					p.advance()
				} else {
					return nil, p.error("expected NULL after NOT")
				}
				action = SetNotNull
			} else {
				// Try to parse as SET NOT NULL without the intermediate keywords
				return nil, p.error("expected DATA TYPE, DEFAULT, or NOT NULL after SET")
			}
		} else if p.matchKeyword("DROP") {
			p.advance()
			if p.matchKeyword("DEFAULT") {
				p.advance()
				action = DropColumnDefault
			} else if p.matchKeyword("NOT") || p.match(TokenNot) {
				p.advance()
				if p.match(TokenNull) || p.matchKeyword("NULL") {
					p.advance()
				} else {
					return nil, p.error("expected NULL after NOT")
				}
				action = DropNotNull
			} else {
				return nil, p.error("expected DEFAULT or NOT NULL after DROP")
			}
		} else if p.matchKeyword("TYPE") {
			p.advance()
			action = SetDataType
		} else {
			return nil, p.error("expected SET, DROP, or TYPE")
		}

		return &AlterColumnAction{Column: columnName, Action: action}, nil
	} else if p.matchKeyword("RENAME") {
		p.advance()

		if p.matchKeyword("TO") {
			p.advance()
			// This would be RENAME TABLE TO new_name, but we're in ALTER TABLE context
			// so this might be RENAME COLUMN old_name TO new_name
			return nil, p.error("RENAME TO not implemented in this context")
		} else if p.matchKeyword("COLUMN") {
			p.advance()
			// RENAME COLUMN old_name TO new_name
			return nil, p.error("RENAME COLUMN not implemented")
		} else {
			return nil, p.error("expected TO or COLUMN after RENAME")
		}
	} else {
		return nil, p.error("expected ALTER TABLE action")
	}
}

// Transaction statement parsing methods

// parseBeginTransactionStatement parses BEGIN/START TRANSACTION statements
func (p *Parser) parseBeginTransactionStatement() (*BeginTransactionStatement, error) {
	stmt := &BeginTransactionStatement{}

	// Consume BEGIN or START
	if p.matchKeyword("BEGIN") {
		p.advance()
	} else if p.matchKeyword("START") {
		p.advance()
		if err := p.consumeKeyword("TRANSACTION", "expected TRANSACTION after START"); err != nil {
			return nil, err
		}
	}

	// Optional TRANSACTION keyword after BEGIN
	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	// Parse optional transaction characteristics
	for {
		if p.matchKeyword("ISOLATION") {
			p.advance()
			if err := p.consumeKeyword("LEVEL", "expected LEVEL after ISOLATION"); err != nil {
				return nil, err
			}

			isolationLevel, err := p.parseIsolationLevel()
			if err != nil {
				return nil, err
			}
			stmt.IsolationLevel = &isolationLevel

		} else if p.matchKeyword("READ") {
			p.advance()
			if p.matchKeyword("ONLY") {
				p.advance()
				stmt.ReadOnly = true
			} else if p.matchKeyword("WRITE") {
				p.advance()
				stmt.ReadOnly = false
			} else {
				return nil, p.error("expected ONLY or WRITE after READ")
			}

		} else if p.matchKeyword("DEFERRABLE") {
			p.advance()
			stmt.Deferrable = true

		} else if p.matchKeyword("NOT") {
			p.advance()
			if err := p.consumeKeyword("DEFERRABLE", "expected DEFERRABLE after NOT"); err != nil {
				return nil, err
			}
			stmt.Deferrable = false

		} else {
			break
		}

		// Optional comma between characteristics
		if p.match(TokenComma) {
			p.advance()
		}
	}

	return stmt, nil
}

// parseCommitTransactionStatement parses COMMIT statements
func (p *Parser) parseCommitTransactionStatement() (*CommitTransactionStatement, error) {
	stmt := &CommitTransactionStatement{}

	// Consume COMMIT
	p.advance()

	// Optional TRANSACTION keyword
	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	// Optional AND CHAIN
	if p.matchKeyword("AND") {
		p.advance()
		if err := p.consumeKeyword("CHAIN", "expected CHAIN after AND"); err != nil {
			return nil, err
		}
		stmt.Chain = true
	}

	return stmt, nil
}

// parseRollbackTransactionStatement parses ROLLBACK statements
func (p *Parser) parseRollbackTransactionStatement() (*RollbackTransactionStatement, error) {
	stmt := &RollbackTransactionStatement{}

	// Consume ROLLBACK
	p.advance()

	// Optional TRANSACTION keyword
	if p.matchKeyword("TRANSACTION") {
		p.advance()
	}

	// Optional TO SAVEPOINT
	if p.matchKeyword("TO") {
		p.advance()
		if err := p.consumeKeyword("SAVEPOINT", "expected SAVEPOINT after TO"); err != nil {
			return nil, err
		}

		if !p.match(TokenIdentifier) {
			return nil, p.error("expected savepoint name")
		}
		stmt.Savepoint = p.current.Value
		p.advance()
	}

	// Optional AND CHAIN
	if p.matchKeyword("AND") {
		p.advance()
		if err := p.consumeKeyword("CHAIN", "expected CHAIN after AND"); err != nil {
			return nil, err
		}
		stmt.Chain = true
	}

	return stmt, nil
}

// parseSavepointStatement parses SAVEPOINT statements
func (p *Parser) parseSavepointStatement() (*SavepointStatement, error) {
	stmt := &SavepointStatement{}

	// Consume SAVEPOINT
	p.advance()

	// Parse savepoint name
	if !p.match(TokenIdentifier) {
		return nil, p.error("expected savepoint name")
	}
	stmt.Name = p.current.Value
	p.advance()

	return stmt, nil
}

// parseReleaseSavepointStatement parses RELEASE SAVEPOINT statements
func (p *Parser) parseReleaseSavepointStatement() (*ReleaseSavepointStatement, error) {
	stmt := &ReleaseSavepointStatement{}

	// Consume RELEASE
	p.advance()

	// Consume SAVEPOINT
	if err := p.consumeKeyword("SAVEPOINT", "expected SAVEPOINT after RELEASE"); err != nil {
		return nil, err
	}

	// Parse savepoint name
	if !p.match(TokenIdentifier) {
		return nil, p.error("expected savepoint name")
	}
	stmt.Name = p.current.Value
	p.advance()

	return stmt, nil
}

// parseIsolationLevel parses isolation level specifications
func (p *Parser) parseIsolationLevel() (SQLIsolationLevel, error) {
	if p.matchKeyword("READ") {
		p.advance()
		if p.matchKeyword("UNCOMMITTED") {
			p.advance()
			return SQLReadUncommitted, nil
		} else if p.matchKeyword("COMMITTED") {
			p.advance()
			return SQLReadCommitted, nil
		} else {
			return 0, p.error("expected UNCOMMITTED or COMMITTED after READ")
		}
	} else if p.matchKeyword("REPEATABLE") {
		p.advance()
		if err := p.consumeKeyword("READ", "expected READ after REPEATABLE"); err != nil {
			return 0, err
		}
		return SQLRepeatableRead, nil
	} else if p.matchKeyword("SERIALIZABLE") {
		p.advance()
		return SQLSerializable, nil
	} else {
		return 0, p.error("expected isolation level (READ UNCOMMITTED, READ COMMITTED, REPEATABLE READ, or SERIALIZABLE)")
	}
}
