package sql

import (
	"fmt"
	"strings"
	"testing"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:  "simple select",
			input: "SELECT * FROM users",
			expected: []TokenType{
				TokenKeyword, TokenMultiply, TokenKeyword, TokenIdentifier, TokenEOF,
			},
		},
		{
			name:  "string literal",
			input: "SELECT 'hello world'",
			expected: []TokenType{
				TokenKeyword, TokenString, TokenEOF,
			},
		},
		{
			name:  "numbers",
			input: "SELECT 123, 45.67",
			expected: []TokenType{
				TokenKeyword, TokenInteger, TokenComma, TokenFloat, TokenEOF,
			},
		},
		{
			name:  "operators",
			input: "SELECT a = b AND c <> d",
			expected: []TokenType{
				TokenKeyword, TokenIdentifier, TokenEqual, TokenIdentifier,
				TokenAnd, TokenIdentifier, TokenNotEqual, TokenIdentifier, TokenEOF,
			},
		},
		{
			name:  "complex operators",
			input: "SELECT a || b, c -> d, e ->> f",
			expected: []TokenType{
				TokenKeyword, TokenIdentifier, TokenConcat, TokenIdentifier, TokenComma,
				TokenIdentifier, TokenJsonExtract, TokenIdentifier, TokenComma,
				TokenIdentifier, TokenJsonExtractText, TokenIdentifier, TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := TokenizeSQL(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tokens) != len(tt.expected) {
				t.Fatalf("expected %d tokens, got %d", len(tt.expected), len(tokens))
			}

			for i, expected := range tt.expected {
				if tokens[i].Type != expected {
					t.Errorf("token %d: expected %v, got %v", i, expected, tokens[i].Type)
				}
			}
		})
	}
}

func TestParseSimpleSelect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "basic select",
			input: "SELECT * FROM users",
			valid: true,
		},
		{
			name:  "select with columns",
			input: "SELECT id, name, email FROM users",
			valid: true,
		},
		{
			name:  "select with where",
			input: "SELECT * FROM users WHERE id = 1",
			valid: true,
		},
		{
			name:  "select with order by",
			input: "SELECT * FROM users ORDER BY name ASC",
			valid: true,
		},
		{
			name:  "select with limit",
			input: "SELECT * FROM users LIMIT 10",
			valid: true,
		},
		{
			name:  "select with distinct",
			input: "SELECT DISTINCT name FROM users",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if selectStmt == nil {
					t.Fatalf("got nil SelectStatement")
				}
			}
		})
	}
}

func TestParseComplexSelect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "select with join",
			input: `SELECT u.name, p.title 
					FROM users u 
					INNER JOIN posts p ON u.id = p.user_id`,
			valid: true,
		},
		{
			name: "select with subquery",
			input: `SELECT * FROM users 
					WHERE id IN (SELECT user_id FROM posts WHERE published = true)`,
			valid: true,
		},
		{
			name: "select with case expression",
			input: `SELECT name, 
					CASE 
						WHEN age < 18 THEN 'minor'
						WHEN age >= 18 THEN 'adult'
						ELSE 'unknown'
					END as age_group
					FROM users`,
			valid: true,
		},
		{
			name: "select with window function",
			input: `SELECT name, 
					ROW_NUMBER() OVER (ORDER BY created_at) as row_num
					FROM users`,
			valid: true,
		},
		{
			name: "select with CTE",
			input: `WITH active_users AS (
						SELECT * FROM users WHERE active = true
					)
					SELECT * FROM active_users`,
			valid: true,
		},
		{
			name: "select with group by and having",
			input: `SELECT department, COUNT(*) as emp_count
					FROM employees 
					GROUP BY department 
					HAVING COUNT(*) > 5`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if selectStmt == nil {
					t.Fatalf("got nil SelectStatement")
				}
			}
		})
	}
}

func TestParseExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "arithmetic expression",
			input: "SELECT a + b * c FROM table1",
			valid: true,
		},
		{
			name:  "comparison expression",
			input: "SELECT * FROM table1 WHERE a > b AND c <= d",
			valid: true,
		},
		{
			name:  "like expression",
			input: "SELECT * FROM table1 WHERE name LIKE '%john%'",
			valid: true,
		},
		{
			name:  "in expression",
			input: "SELECT * FROM table1 WHERE id IN (1, 2, 3)",
			valid: true,
		},
		{
			name:  "exists expression",
			input: "SELECT * FROM table1 WHERE EXISTS (SELECT 1 FROM table2 WHERE table2.id = table1.id)",
			valid: true,
		},
		{
			name:  "function call",
			input: "SELECT UPPER(name), LENGTH(description) FROM table1",
			valid: true,
		},
		{
			name:  "aggregate function",
			input: "SELECT COUNT(*), SUM(amount) FROM table1",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if len(selectStmt.Fields) == 0 {
					t.Fatalf("expected at least one field")
				}
			}
		})
	}
}

func TestParseJoins(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "inner join",
			input: "SELECT * FROM a INNER JOIN b ON a.id = b.a_id",
			valid: true,
		},
		{
			name:  "left join",
			input: "SELECT * FROM a LEFT JOIN b ON a.id = b.a_id",
			valid: true,
		},
		{
			name:  "right join",
			input: "SELECT * FROM a RIGHT JOIN b ON a.id = b.a_id",
			valid: true,
		},
		{
			name:  "full join",
			input: "SELECT * FROM a FULL JOIN b ON a.id = b.a_id",
			valid: true,
		},
		{
			name:  "cross join",
			input: "SELECT * FROM a CROSS JOIN b",
			valid: true,
		},
		{
			name:  "natural join",
			input: "SELECT * FROM a NATURAL JOIN b",
			valid: true,
		},
		{
			name:  "join with using",
			input: "SELECT * FROM a JOIN b USING (id)",
			valid: true,
		},
		{
			name:  "multiple joins",
			input: "SELECT * FROM a JOIN b ON a.id = b.a_id JOIN c ON b.id = c.b_id",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if len(selectStmt.From) == 0 {
					t.Fatalf("expected at least one table in FROM clause")
				}
			}
		})
	}
}

func TestParseWindowFunctions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "row number",
			input: "SELECT ROW_NUMBER() OVER (ORDER BY id) FROM table1",
			valid: true,
		},
		{
			name:  "rank with partition",
			input: "SELECT RANK() OVER (PARTITION BY department ORDER BY salary DESC) FROM employees",
			valid: true,
		},
		{
			name:  "lag function",
			input: "SELECT LAG(salary, 1) OVER (ORDER BY hire_date) FROM employees",
			valid: true,
		},
		{
			name:  "window with frame",
			input: "SELECT SUM(amount) OVER (ORDER BY date ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM transactions",
			valid: true,
		},
		{
			name:  "named window",
			input: "SELECT SUM(amount) OVER w FROM transactions WINDOW w AS (ORDER BY date)",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if len(selectStmt.Fields) == 0 {
					t.Fatalf("expected at least one field")
				}
			}
		})
	}
}

func TestParseCTE(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "simple CTE",
			input: `WITH cte AS (SELECT * FROM table1) 
					SELECT * FROM cte`,
			valid: true,
		},
		{
			name: "CTE with columns",
			input: `WITH cte(id, name) AS (SELECT id, name FROM table1) 
					SELECT * FROM cte`,
			valid: true,
		},
		{
			name: "multiple CTEs",
			input: `WITH 
					cte1 AS (SELECT * FROM table1),
					cte2 AS (SELECT * FROM table2)
					SELECT * FROM cte1 JOIN cte2 ON cte1.id = cte2.id`,
			valid: true,
		},
		{
			name: "recursive CTE",
			input: `WITH RECURSIVE cte AS (
						SELECT 1 as n
						UNION ALL
						SELECT n + 1 FROM cte WHERE n < 10
					)
					SELECT * FROM cte`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if tt.valid && err != nil {
				t.Fatalf("expected valid SQL, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("expected invalid SQL, got no error")
			}

			if tt.valid {
				selectStmt, ok := stmt.(*SelectStatement)
				if !ok {
					t.Fatalf("expected SelectStatement, got %T", stmt)
				}
				if len(selectStmt.With) == 0 {
					t.Fatalf("expected at least one CTE")
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing FROM",
			input: "SELECT *",
		},
		{
			name:  "invalid token",
			input: "SELECT @ FROM table1",
		},
		{
			name:  "unterminated string",
			input: "SELECT 'unterminated FROM table1",
		},
		{
			name:  "missing closing paren",
			input: "SELECT * FROM table1 WHERE id IN (1, 2, 3",
		},
		{
			name:  "invalid join syntax",
			input: "SELECT * FROM a JOIN ON a.id = b.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSQL(tt.input)
			if err == nil {
				t.Fatalf("expected error for invalid SQL: %s", tt.input)
			}

			// Check that error contains useful information
			if !strings.Contains(err.Error(), "parse error") &&
				!strings.Contains(err.Error(), "lexer error") {
				t.Errorf("error should contain 'parse error' or 'lexer error': %v", err)
			}
		})
	}
}

func TestValidator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
	}{
		{
			name:     "valid select",
			input:    "SELECT id, name FROM users",
			hasError: false,
		},
		{
			name:     "unknown function",
			input:    "SELECT UNKNOWN_FUNC(id) FROM users",
			hasError: true,
		},
		{
			name:     "aggregate without group by",
			input:    "SELECT name, COUNT(*) FROM users",
			hasError: false, // This should be a warning, not an error
		},
		{
			name:     "window function without OVER",
			input:    "SELECT ROW_NUMBER() FROM users",
			hasError: true,
		},
		{
			name:     "invalid argument count",
			input:    "SELECT UPPER() FROM users",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseSQL(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			validator := NewValidator()
			err = validator.Validate(stmt)

			if tt.hasError && err == nil {
				t.Fatalf("expected validation error")
			}
			if !tt.hasError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestComplexQueries(t *testing.T) {
	complexQueries := []string{
		// Complex SELECT with multiple JOINs, subqueries, and window functions
		`WITH regional_sales AS (
			SELECT region, SUM(sales_amount) as total_sales
			FROM sales_data 
			WHERE sale_date >= '2023-01-01'
			GROUP BY region
		),
		top_regions AS (
			SELECT region
			FROM regional_sales
			WHERE total_sales > (SELECT AVG(total_sales) * 1.5 FROM regional_sales)
		)
		SELECT 
			s.salesperson_name,
			s.region,
			s.sales_amount,
			RANK() OVER (PARTITION BY s.region ORDER BY s.sales_amount DESC) as region_rank,
			LAG(s.sales_amount, 1) OVER (PARTITION BY s.region ORDER BY s.sale_date) as prev_sale,
			CASE 
				WHEN s.sales_amount > 10000 THEN 'High'
				WHEN s.sales_amount > 5000 THEN 'Medium'
				ELSE 'Low'
			END as performance_tier
		FROM sales_data s
		INNER JOIN top_regions tr ON s.region = tr.region
		LEFT JOIN customers c ON s.customer_id = c.id
		WHERE s.sale_date BETWEEN '2023-01-01' AND '2023-12-31'
			AND s.sales_amount > 1000
			AND c.status = 'active'
		ORDER BY s.region, s.sales_amount DESC
		LIMIT 100`,

		// Complex query with multiple CTEs and window functions
		`WITH RECURSIVE employee_hierarchy AS (
			SELECT employee_id, name, manager_id, 1 as level
			FROM employees
			WHERE manager_id IS NULL
			
			UNION ALL
			
			SELECT e.employee_id, e.name, e.manager_id, eh.level + 1
			FROM employees e
			INNER JOIN employee_hierarchy eh ON e.manager_id = eh.employee_id
		),
		department_stats AS (
			SELECT 
				department_id,
				COUNT(*) as employee_count,
				AVG(salary) as avg_salary,
				STDDEV(salary) as salary_stddev
			FROM employees
			GROUP BY department_id
		)
		SELECT 
			eh.name,
			eh.level,
			d.department_name,
			e.salary,
			ds.avg_salary,
			(e.salary - ds.avg_salary) / ds.salary_stddev as salary_z_score,
			DENSE_RANK() OVER (PARTITION BY e.department_id ORDER BY e.salary DESC) as dept_salary_rank
		FROM employee_hierarchy eh
		JOIN employees e ON eh.employee_id = e.employee_id
		JOIN departments d ON e.department_id = d.department_id
		JOIN department_stats ds ON e.department_id = ds.department_id
		WHERE eh.level <= 5
		ORDER BY eh.level, e.department_id, e.salary DESC`,
	}

	for i, query := range complexQueries {
		t.Run(fmt.Sprintf("complex_query_%d", i+1), func(t *testing.T) {
			stmt, err := ParseSQL(query)
			if err != nil {
				t.Fatalf("failed to parse complex query: %v", err)
			}

			selectStmt, ok := stmt.(*SelectStatement)
			if !ok {
				t.Fatalf("expected SelectStatement, got %T", stmt)
			}

			// Basic validation that the query was parsed correctly
			if len(selectStmt.Fields) == 0 {
				t.Fatalf("expected at least one field in SELECT")
			}

			if len(selectStmt.From) == 0 {
				t.Fatalf("expected at least one table in FROM")
			}

			// Validate the query
			validator := NewValidator()
			err = validator.Validate(stmt)
			if err != nil {
				// For complex queries, we might have warnings but not errors
				errors := validator.GetErrors()
				if len(errors) > 0 {
					t.Logf("validation errors: %v", errors)
				}
				warnings := validator.GetWarnings()
				if len(warnings) > 0 {
					t.Logf("validation warnings: %v", warnings)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkLexer(b *testing.B) {
	query := `SELECT u.id, u.name, p.title, COUNT(c.id) as comment_count
			  FROM users u
			  LEFT JOIN posts p ON u.id = p.user_id
			  LEFT JOIN comments c ON p.id = c.post_id
			  WHERE u.active = true AND p.published = true
			  GROUP BY u.id, u.name, p.title
			  ORDER BY comment_count DESC
			  LIMIT 10`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := TokenizeSQL(query)
		if err != nil {
			b.Fatalf("lexer error: %v", err)
		}
	}
}

func BenchmarkParser(b *testing.B) {
	query := `SELECT u.id, u.name, p.title, COUNT(c.id) as comment_count
			  FROM users u
			  LEFT JOIN posts p ON u.id = p.user_id
			  LEFT JOIN comments c ON p.id = c.post_id
			  WHERE u.active = true AND p.published = true
			  GROUP BY u.id, u.name, p.title
			  ORDER BY comment_count DESC
			  LIMIT 10`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseSQL(query)
		if err != nil {
			b.Fatalf("parser error: %v", err)
		}
	}
}

func BenchmarkValidator(b *testing.B) {
	query := `SELECT u.id, u.name, p.title, COUNT(c.id) as comment_count
			  FROM users u
			  LEFT JOIN posts p ON u.id = p.user_id
			  LEFT JOIN comments c ON p.id = c.post_id
			  WHERE u.active = true AND p.published = true
			  GROUP BY u.id, u.name, p.title
			  ORDER BY comment_count DESC
			  LIMIT 10`

	stmt, err := ParseSQL(query)
	if err != nil {
		b.Fatalf("parser error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator := NewValidator()
		validator.Validate(stmt)
	}
}
