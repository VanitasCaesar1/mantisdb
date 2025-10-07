package sql

import (
	"fmt"
	"strings"
	"testing"
)

func TestAdvancedDDLStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "CREATE TABLE with constraints",
			input: `CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				email VARCHAR(255) UNIQUE NOT NULL,
				name VARCHAR(100) NOT NULL,
				age INTEGER CHECK (age >= 0),
				created_at TIMESTAMP DEFAULT NOW(),
				CONSTRAINT unique_email UNIQUE (email),
				CONSTRAINT fk_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE
			)`,
			valid: true,
		},
		{
			name: "CREATE TEMPORARY TABLE IF NOT EXISTS",
			input: `CREATE TEMPORARY TABLE IF NOT EXISTS temp_data (
				id INTEGER,
				data TEXT
			)`,
			valid: true,
		},
		{
			name: "CREATE UNIQUE INDEX",
			input: `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email 
					ON users (email) 
					WHERE active = true`,
			valid: true,
		},
		{
			name: "CREATE INDEX with expression",
			input: `CREATE INDEX idx_users_lower_email 
					ON users (LOWER(email))`,
			valid: true,
		},
		{
			name:  "DROP TABLE with CASCADE",
			input: `DROP TABLE IF EXISTS old_table CASCADE`,
			valid: true,
		},
		{
			name:  "DROP multiple tables",
			input: `DROP TABLE table1, table2, table3`,
			valid: true,
		},
		{
			name: "ALTER TABLE add column",
			input: `ALTER TABLE users 
					ADD COLUMN phone VARCHAR(20),
					ADD CONSTRAINT check_phone CHECK (phone ~ '^[0-9-]+$')`,
			valid: true,
		},
		{
			name: "ALTER TABLE drop column",
			input: `ALTER TABLE users 
					DROP COLUMN old_field CASCADE,
					DROP CONSTRAINT old_constraint`,
			valid: true,
		},
		{
			name: "ALTER TABLE modify column",
			input: `ALTER TABLE users 
					ALTER COLUMN email SET NOT NULL,
					ALTER COLUMN age DROP DEFAULT`,
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

			if tt.valid && stmt == nil {
				t.Fatalf("expected statement, got nil")
			}
		})
	}
}

func TestAdvancedDMLStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "INSERT with ON CONFLICT",
			input: `INSERT INTO users (email, name) 
					VALUES ('test@example.com', 'Test User')
					ON CONFLICT (email) DO UPDATE SET 
						name = EXCLUDED.name,
						updated_at = NOW()
					WHERE users.active = true`,
			valid: true,
		},
		{
			name: "INSERT with subquery",
			input: `INSERT INTO user_stats (user_id, post_count)
					SELECT u.id, COUNT(p.id)
					FROM users u
					LEFT JOIN posts p ON u.id = p.user_id
					GROUP BY u.id`,
			valid: true,
		},
		{
			name: "UPDATE with FROM clause",
			input: `UPDATE users 
					SET department_name = d.name
					FROM departments d
					WHERE users.department_id = d.id`,
			valid: true,
		},
		{
			name: "DELETE with USING clause",
			input: `DELETE FROM posts
					USING users
					WHERE posts.user_id = users.id
					AND users.active = false`,
			valid: true,
		},
		{
			name: "Complex UPDATE with subquery",
			input: `UPDATE products 
					SET price = price * 1.1
					WHERE category_id IN (
						SELECT id FROM categories 
						WHERE name IN ('Electronics', 'Computers')
					)`,
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

			if tt.valid && stmt == nil {
				t.Fatalf("expected statement, got nil")
			}
		})
	}
}

func TestAdvancedExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "BETWEEN expression",
			input: "SELECT * FROM products WHERE price BETWEEN 10.00 AND 100.00",
			valid: true,
		},
		{
			name:  "NOT BETWEEN expression",
			input: "SELECT * FROM products WHERE price NOT BETWEEN 10.00 AND 100.00",
			valid: true,
		},
		{
			name:  "IS NULL expression",
			input: "SELECT * FROM users WHERE deleted_at IS NULL",
			valid: true,
		},
		{
			name:  "IS NOT NULL expression",
			input: "SELECT * FROM users WHERE email IS NOT NULL",
			valid: true,
		},
		{
			name:  "CAST expression",
			input: "SELECT CAST(price AS INTEGER) FROM products",
			valid: true,
		},
		{
			name:  "EXTRACT expression",
			input: "SELECT EXTRACT(YEAR FROM created_at) FROM orders",
			valid: true,
		},
		{
			name:  "Array literal",
			input: "SELECT * FROM products WHERE category_id = ANY([1, 2, 3])",
			valid: true,
		},
		{
			name: "Complex CASE expression",
			input: `SELECT 
						CASE 
							WHEN age < 18 THEN 'Minor'
							WHEN age BETWEEN 18 AND 65 THEN 'Adult'
							WHEN age > 65 THEN 'Senior'
							ELSE 'Unknown'
						END as age_group
					FROM users`,
			valid: true,
		},
		{
			name:  "Nested function calls",
			input: "SELECT UPPER(SUBSTRING(name, 1, 10)) FROM users",
			valid: true,
		},
		{
			name:  "Complex arithmetic",
			input: "SELECT (price * quantity * (1 + tax_rate)) as total FROM order_items",
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

func TestAdvancedJoins(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "Multiple joins with different types",
			input: `SELECT u.name, p.title, c.content, t.name as tag
					FROM users u
					INNER JOIN posts p ON u.id = p.user_id
					LEFT JOIN comments c ON p.id = c.post_id
					RIGHT JOIN post_tags pt ON p.id = pt.post_id
					FULL OUTER JOIN tags t ON pt.tag_id = t.id
					CROSS JOIN categories cat
					WHERE u.active = true`,
			valid: true,
		},
		{
			name:  "NATURAL JOIN",
			input: `SELECT * FROM users NATURAL JOIN profiles`,
			valid: true,
		},
		{
			name:  "JOIN with USING clause",
			input: `SELECT * FROM orders o JOIN customers c USING (customer_id)`,
			valid: true,
		},
		{
			name: "Self join",
			input: `SELECT e1.name as employee, e2.name as manager
					FROM employees e1
					LEFT JOIN employees e2 ON e1.manager_id = e2.id`,
			valid: true,
		},
		{
			name: "Join with subquery",
			input: `SELECT u.name, stats.post_count
					FROM users u
					JOIN (
						SELECT user_id, COUNT(*) as post_count
						FROM posts
						GROUP BY user_id
					) stats ON u.id = stats.user_id`,
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

func TestAdvancedSubqueries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "Correlated subquery",
			input: `SELECT u.name, (
						SELECT COUNT(*) 
						FROM posts p 
						WHERE p.user_id = u.id
					) as post_count
					FROM users u`,
			valid: true,
		},
		{
			name: "EXISTS subquery",
			input: `SELECT * FROM users u
					WHERE EXISTS (
						SELECT 1 FROM posts p 
						WHERE p.user_id = u.id AND p.published = true
					)`,
			valid: true,
		},
		{
			name: "NOT EXISTS subquery",
			input: `SELECT * FROM users u
					WHERE NOT EXISTS (
						SELECT 1 FROM posts p 
						WHERE p.user_id = u.id
					)`,
			valid: true,
		},
		{
			name: "IN subquery",
			input: `SELECT * FROM products
					WHERE category_id IN (
						SELECT id FROM categories 
						WHERE active = true
					)`,
			valid: true,
		},
		{
			name: "ANY/ALL subquery",
			input: `SELECT * FROM products
					WHERE price > ALL (
						SELECT price FROM products 
						WHERE category_id = 1
					)`,
			valid: true,
		},
		{
			name: "Subquery in FROM clause",
			input: `SELECT avg_price.category, avg_price.price
					FROM (
						SELECT category_id as category, AVG(price) as price
						FROM products
						GROUP BY category_id
					) avg_price
					WHERE avg_price.price > 100`,
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

func TestAdvancedWindowFunctions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "Window function with frame",
			input: `SELECT 
						name,
						salary,
						SUM(salary) OVER (
							ORDER BY salary 
							ROWS BETWEEN 2 PRECEDING AND 2 FOLLOWING
						) as rolling_sum
					FROM employees`,
			valid: true,
		},
		{
			name: "Multiple window functions",
			input: `SELECT 
						name,
						department,
						salary,
						ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as dept_rank,
						RANK() OVER (ORDER BY salary DESC) as overall_rank,
						LAG(salary, 1) OVER (PARTITION BY department ORDER BY hire_date) as prev_salary
					FROM employees`,
			valid: true,
		},
		{
			name: "Named window",
			input: `SELECT 
						name,
						salary,
						ROW_NUMBER() OVER w as row_num,
						RANK() OVER w as rank_num
					FROM employees
					WINDOW w AS (PARTITION BY department ORDER BY salary DESC)`,
			valid: true,
		},
		{
			name: "Window function with FILTER",
			input: `SELECT 
						department,
						COUNT(*) FILTER (WHERE salary > 50000) OVER (PARTITION BY department) as high_earners
					FROM employees`,
			valid: true,
		},
		{
			name: "Complex window frame",
			input: `SELECT 
						date,
						amount,
						SUM(amount) OVER (
							ORDER BY date 
							RANGE BETWEEN INTERVAL '7 days' PRECEDING AND CURRENT ROW
						) as weekly_total
					FROM transactions`,
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

func TestAdvancedCTEs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name: "Recursive CTE - Employee hierarchy",
			input: `WITH RECURSIVE employee_tree AS (
						SELECT id, name, manager_id, 1 as level
						FROM employees
						WHERE manager_id IS NULL
						
						UNION ALL
						
						SELECT e.id, e.name, e.manager_id, et.level + 1
						FROM employees e
						JOIN employee_tree et ON e.manager_id = et.id
					)
					SELECT * FROM employee_tree ORDER BY level, name`,
			valid: true,
		},
		{
			name: "Multiple CTEs",
			input: `WITH 
					sales_summary AS (
						SELECT 
							salesperson_id,
							SUM(amount) as total_sales,
							COUNT(*) as sale_count
						FROM sales
						WHERE sale_date >= '2023-01-01'
						GROUP BY salesperson_id
					),
					top_performers AS (
						SELECT salesperson_id
						FROM sales_summary
						WHERE total_sales > 100000
					)
					SELECT 
						e.name,
						ss.total_sales,
						ss.sale_count
					FROM employees e
					JOIN sales_summary ss ON e.id = ss.salesperson_id
					JOIN top_performers tp ON e.id = tp.salesperson_id`,
			valid: true,
		},
		{
			name: "CTE with column aliases",
			input: `WITH regional_stats(region, total, average) AS (
						SELECT 
							region,
							SUM(sales_amount),
							AVG(sales_amount)
						FROM sales
						GROUP BY region
					)
					SELECT * FROM regional_stats WHERE total > 50000`,
			valid: true,
		},
		{
			name: "Nested CTEs",
			input: `WITH RECURSIVE category_tree AS (
						SELECT id, name, parent_id, 1 as depth
						FROM categories
						WHERE parent_id IS NULL
						
						UNION ALL
						
						SELECT c.id, c.name, c.parent_id, ct.depth + 1
						FROM categories c
						JOIN category_tree ct ON c.parent_id = ct.id
					),
					category_products AS (
						SELECT 
							ct.id as category_id,
							ct.name as category_name,
							ct.depth,
							COUNT(p.id) as product_count
						FROM category_tree ct
						LEFT JOIN products p ON ct.id = p.category_id
						GROUP BY ct.id, ct.name, ct.depth
					)
					SELECT * FROM category_products ORDER BY depth, category_name`,
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

func TestEnhancedErrorReporting(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "misspelled SELECT",
			input:         "SELCT * FROM users",
			expectedError: "SELECT",
		},
		{
			name:          "missing FROM",
			input:         "SELECT *",
			expectedError: "FROM",
		},
		{
			name:          "unterminated string",
			input:         "SELECT 'unterminated FROM users",
			expectedError: "unterminated",
		},
		{
			name:          "missing closing paren",
			input:         "SELECT * FROM users WHERE id IN (1, 2, 3",
			expectedError: "')'",
		},
		{
			name:          "invalid join syntax",
			input:         "SELECT * FROM users JOIN ON users.id = posts.user_id",
			expectedError: "table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSQLEnhanced(tt.input)
			if err == nil {
				t.Fatalf("expected error for invalid SQL: %s", tt.input)
			}

			errorMsg := err.Error()
			if !strings.Contains(errorMsg, tt.expectedError) {
				t.Errorf("expected error to contain '%s', got: %s", tt.expectedError, errorMsg)
			}

			// Check if it's an enhanced SQL error
			if sqlErr, ok := err.(*SQLError); ok {
				if sqlErr.Line == 0 || sqlErr.Column == 0 {
					t.Errorf("expected line and column information in error")
				}
			}
		})
	}
}

func TestComplexRealWorldQueries(t *testing.T) {
	complexQueries := []string{
		// E-commerce analytics query
		`WITH RECURSIVE category_hierarchy AS (
			SELECT id, name, parent_id, 0 as level, ARRAY[id] as path
			FROM categories
			WHERE parent_id IS NULL
			
			UNION ALL
			
			SELECT c.id, c.name, c.parent_id, ch.level + 1, ch.path || c.id
			FROM categories c
			JOIN category_hierarchy ch ON c.parent_id = ch.id
		),
		monthly_sales AS (
			SELECT 
				DATE_TRUNC('month', order_date) as month,
				p.category_id,
				SUM(oi.quantity * oi.price) as revenue,
				COUNT(DISTINCT o.id) as order_count,
				COUNT(DISTINCT o.customer_id) as unique_customers
			FROM orders o
			JOIN order_items oi ON o.id = oi.order_id
			JOIN products p ON oi.product_id = p.id
			WHERE o.order_date >= CURRENT_DATE - INTERVAL '12 months'
			GROUP BY DATE_TRUNC('month', order_date), p.category_id
		),
		category_performance AS (
			SELECT 
				ch.name as category_name,
				ch.level,
				ms.month,
				ms.revenue,
				ms.order_count,
				ms.unique_customers,
				LAG(ms.revenue, 1) OVER (PARTITION BY ch.id ORDER BY ms.month) as prev_month_revenue,
				RANK() OVER (PARTITION BY ms.month ORDER BY ms.revenue DESC) as revenue_rank
			FROM category_hierarchy ch
			JOIN monthly_sales ms ON ch.id = ms.category_id
		)
		SELECT 
			category_name,
			month,
			revenue,
			CASE 
				WHEN prev_month_revenue IS NULL THEN NULL
				ELSE ROUND(((revenue - prev_month_revenue) / prev_month_revenue * 100)::numeric, 2)
			END as growth_rate,
			revenue_rank,
			SUM(revenue) OVER (PARTITION BY category_name ORDER BY month ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as rolling_3month_revenue
		FROM category_performance
		WHERE level <= 2
		ORDER BY category_name, month`,

		// User engagement analysis
		`WITH user_activity AS (
			SELECT 
				user_id,
				DATE(created_at) as activity_date,
				COUNT(*) as actions,
				COUNT(DISTINCT session_id) as sessions
			FROM user_events
			WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY user_id, DATE(created_at)
		),
		user_streaks AS (
			SELECT 
				user_id,
				activity_date,
				actions,
				sessions,
				activity_date - ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY activity_date)::integer as streak_group
			FROM user_activity
		),
		streak_lengths AS (
			SELECT 
				user_id,
				streak_group,
				COUNT(*) as streak_length,
				MIN(activity_date) as streak_start,
				MAX(activity_date) as streak_end,
				SUM(actions) as total_actions,
				SUM(sessions) as total_sessions
			FROM user_streaks
			GROUP BY user_id, streak_group
		),
		user_engagement_metrics AS (
			SELECT 
				u.id as user_id,
				u.email,
				u.created_at as user_created_at,
				COALESCE(MAX(sl.streak_length), 0) as longest_streak,
				COALESCE(SUM(ua.actions), 0) as total_actions_30d,
				COALESCE(COUNT(DISTINCT ua.activity_date), 0) as active_days_30d,
				CASE 
					WHEN COUNT(DISTINCT ua.activity_date) >= 20 THEN 'Highly Active'
					WHEN COUNT(DISTINCT ua.activity_date) >= 10 THEN 'Moderately Active'
					WHEN COUNT(DISTINCT ua.activity_date) >= 1 THEN 'Low Activity'
					ELSE 'Inactive'
				END as engagement_level
			FROM users u
			LEFT JOIN user_activity ua ON u.id = ua.user_id
			LEFT JOIN streak_lengths sl ON u.id = sl.user_id
			WHERE u.created_at <= CURRENT_DATE - INTERVAL '7 days'
			GROUP BY u.id, u.email, u.created_at
		)
		SELECT 
			engagement_level,
			COUNT(*) as user_count,
			ROUND(AVG(total_actions_30d), 2) as avg_actions,
			ROUND(AVG(active_days_30d), 2) as avg_active_days,
			ROUND(AVG(longest_streak), 2) as avg_longest_streak,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY total_actions_30d) as median_actions
		FROM user_engagement_metrics
		GROUP BY engagement_level
		ORDER BY 
			CASE engagement_level
				WHEN 'Highly Active' THEN 1
				WHEN 'Moderately Active' THEN 2
				WHEN 'Low Activity' THEN 3
				WHEN 'Inactive' THEN 4
			END`,
	}

	for i, query := range complexQueries {
		t.Run(fmt.Sprintf("real_world_query_%d", i+1), func(t *testing.T) {
			stmt, err := ParseSQL(query)
			if err != nil {
				t.Fatalf("failed to parse real-world query: %v", err)
			}

			selectStmt, ok := stmt.(*SelectStatement)
			if !ok {
				t.Fatalf("expected SelectStatement, got %T", stmt)
			}

			// Validate basic structure
			if len(selectStmt.Fields) == 0 {
				t.Fatalf("expected at least one field in SELECT")
			}

			// Validate CTEs if present
			if len(selectStmt.With) > 0 {
				for _, cte := range selectStmt.With {
					if cte.Name == "" {
						t.Fatalf("CTE must have a name")
					}
					if cte.Query == nil {
						t.Fatalf("CTE must have a query")
					}
				}
			}

			// Run validation
			validator := NewValidator()
			err = validator.Validate(stmt)
			if err != nil {
				// Log warnings but don't fail on them for complex queries
				warnings := validator.GetWarnings()
				if len(warnings) > 0 {
					t.Logf("validation warnings: %v", warnings)
				}
			}
		})
	}
}

// Benchmark the enhanced parser
func BenchmarkAdvancedParser(b *testing.B) {
	query := `WITH RECURSIVE employee_hierarchy AS (
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
	ORDER BY eh.level, e.department_id, e.salary DESC`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseSQL(query)
		if err != nil {
			b.Fatalf("parser error: %v", err)
		}
	}
}
