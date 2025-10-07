//go:build ignore
// +build ignore

package sql

import (
	"fmt"
	"testing"
	"time"
)

func TestCParserBasicQueries(t *testing.T) {
	tests := []struct {
		name  string
		query string
		valid bool
	}{
		{
			name:  "simple select",
			query: "SELECT * FROM users",
			valid: true,
		},
		{
			name:  "select with where",
			query: "SELECT id, name FROM users WHERE age > 18",
			valid: true,
		},
		{
			name: "complex join",
			query: `SELECT u.name, p.title 
					FROM users u 
					INNER JOIN posts p ON u.id = p.user_id 
					WHERE u.active = true`,
			valid: true,
		},
		{
			name: "window function",
			query: `SELECT name, 
					ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank
					FROM employees`,
			valid: true,
		},
		{
			name: "CTE with recursion",
			query: `WITH RECURSIVE employee_hierarchy AS (
						SELECT id, name, manager_id, 1 as level
						FROM employees
						WHERE manager_id IS NULL
						UNION ALL
						SELECT e.id, e.name, e.manager_id, eh.level + 1
						FROM employees e
						JOIN employee_hierarchy eh ON e.manager_id = eh.id
					)
					SELECT * FROM employee_hierarchy`,
			valid: true,
		},
		{
			name: "complex aggregation",
			query: `SELECT 
						department,
						COUNT(*) as employee_count,
						AVG(salary) as avg_salary,
						PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY salary) as median_salary
					FROM employees
					GROUP BY department
					HAVING COUNT(*) > 5
					ORDER BY avg_salary DESC`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := ParseSQLWithC(tt.query)

			if tt.valid && err != nil {
				t.Errorf("expected valid query, got error: %v", err)
			}

			if !tt.valid && err == nil {
				t.Errorf("expected invalid query, got no error")
			}

			if tt.valid && len(tokens) == 0 {
				t.Errorf("expected tokens, got empty result")
			}
		})
	}
}

func TestCParserPerformance(t *testing.T) {
	complexQuery := `
		WITH RECURSIVE category_tree AS (
			SELECT id, name, parent_id, 1 as depth, ARRAY[id] as path
			FROM categories
			WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, c.name, c.parent_id, ct.depth + 1, ct.path || c.id
			FROM categories c
			JOIN category_tree ct ON c.parent_id = ct.id
			WHERE ct.depth < 10
		),
		sales_summary AS (
			SELECT 
				p.category_id,
				DATE_TRUNC('month', s.sale_date) as month,
				SUM(s.amount) as total_sales,
				COUNT(*) as sale_count,
				AVG(s.amount) as avg_sale,
				STDDEV(s.amount) as stddev_sale
			FROM sales s
			JOIN products p ON s.product_id = p.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '12 months'
			GROUP BY p.category_id, DATE_TRUNC('month', s.sale_date)
		),
		category_performance AS (
			SELECT 
				ct.name as category_name,
				ct.depth,
				ss.month,
				ss.total_sales,
				ss.sale_count,
				ss.avg_sale,
				ss.stddev_sale,
				LAG(ss.total_sales, 1) OVER (
					PARTITION BY ct.id 
					ORDER BY ss.month
				) as prev_month_sales,
				RANK() OVER (
					PARTITION BY ss.month 
					ORDER BY ss.total_sales DESC
				) as monthly_rank,
				SUM(ss.total_sales) OVER (
					PARTITION BY ct.id 
					ORDER BY ss.month 
					ROWS BETWEEN 2 PRECEDING AND CURRENT ROW
				) as rolling_3month_sales
			FROM category_tree ct
			JOIN sales_summary ss ON ct.id = ss.category_id
		)
		SELECT 
			category_name,
			month,
			total_sales,
			CASE 
				WHEN prev_month_sales IS NULL THEN NULL
				ELSE ROUND(
					((total_sales - prev_month_sales) / prev_month_sales * 100)::numeric, 
					2
				)
			END as growth_rate,
			monthly_rank,
			rolling_3month_sales,
			NTILE(4) OVER (
				PARTITION BY month 
				ORDER BY total_sales
			) as quartile
		FROM category_performance
		WHERE depth <= 3
		ORDER BY month DESC, total_sales DESC
		LIMIT 100`

	// Benchmark C parser
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, err := ParseSQLWithC(complexQuery)
		if err != nil {
			t.Fatalf("C parser failed: %v", err)
		}
	}
	cParserTime := time.Since(start)

	// Benchmark Go parser
	start = time.Now()
	for i := 0; i < 1000; i++ {
		_, err := ParseSQL(complexQuery)
		if err != nil {
			// Go parser might not support all features
			continue
		}
	}
	goParserTime := time.Since(start)

	t.Logf("C parser time: %v", cParserTime)
	t.Logf("Go parser time: %v", goParserTime)

	if cParserTime > 0 && goParserTime > 0 {
		speedup := float64(goParserTime) / float64(cParserTime)
		t.Logf("C parser speedup: %.2fx", speedup)

		if speedup < 2.0 {
			t.Logf("Warning: C parser speedup is less than 2x")
		}
	}
}

func TestCostEstimation(t *testing.T) {
	tests := []struct {
		name      string
		function  func() float64
		expected  float64
		tolerance float64
	}{
		{
			name: "sequential scan cost",
			function: func() float64 {
				return CostSeqScan(100, 10000) // 100 pages, 10k tuples
			},
			expected:  200, // 100 * 1.0 + 10000 * 0.01
			tolerance: 10,
		},
		{
			name: "index scan cost",
			function: func() float64 {
				return CostIndex(100, 10000, 0.1) // 10% selectivity
			},
			expected:  450, // Approximate
			tolerance: 50,
		},
		{
			name: "hash join cost",
			function: func() float64 {
				return CostHashJoin(100, 200, 1000, 2000)
			},
			expected:  310, // Approximate
			tolerance: 50,
		},
		{
			name: "sort cost",
			function: func() float64 {
				return CostSort(10000, 100) // 10k tuples, 100 bytes each
			},
			expected:  332, // Approximate log-based cost
			tolerance: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.function()

			if actual < tt.expected-tt.tolerance || actual > tt.expected+tt.tolerance {
				t.Errorf("cost estimation out of range: got %.2f, expected %.2f ± %.2f",
					actual, tt.expected, tt.tolerance)
			}
		})
	}
}

func TestQueryOptimization(t *testing.T) {
	queries := []struct {
		name  string
		query string
		stats []*CTableStats
	}{
		{
			name:  "simple select with stats",
			query: "SELECT * FROM users WHERE age > 25",
			stats: []*CTableStats{
				{
					TableName:   "users",
					ColumnName:  "age",
					NTuples:     10000,
					NDistinct:   50,
					Selectivity: 0.3,
					HasIndex:    true,
					IndexPages:  10,
					TablePages:  100,
				},
			},
		},
		{
			name:  "join with statistics",
			query: "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			stats: []*CTableStats{
				{
					TableName:   "users",
					ColumnName:  "id",
					NTuples:     10000,
					NDistinct:   10000,
					Selectivity: 1.0,
					HasIndex:    true,
					IndexPages:  20,
					TablePages:  100,
				},
				{
					TableName:   "orders",
					ColumnName:  "user_id",
					NTuples:     50000,
					NDistinct:   8000,
					Selectivity: 0.8,
					HasIndex:    true,
					IndexPages:  50,
					TablePages:  500,
				},
			},
		},
	}

	for _, tt := range queries {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := OptimizeSQLWithC(tt.query, tt.stats)
			if err != nil {
				t.Fatalf("optimization failed: %v", err)
			}

			if plan == nil {
				t.Fatal("expected plan, got nil")
			}

			if plan.TotalCost <= 0 {
				t.Errorf("expected positive cost, got %.2f", plan.TotalCost)
			}

			if plan.PlanRows <= 0 {
				t.Errorf("expected positive row estimate, got %.2f", plan.PlanRows)
			}

			t.Logf("Plan cost: %.2f, rows: %.0f, width: %d",
				plan.TotalCost, plan.PlanRows, plan.PlanWidth)
		})
	}
}

func TestStatisticsCollection(t *testing.T) {
	tableName := "test_table"

	stats, err := CollectTableStats(tableName)
	if err != nil {
		t.Fatalf("failed to collect stats: %v", err)
	}

	if len(stats) == 0 {
		t.Fatal("expected statistics, got empty result")
	}

	for _, stat := range stats {
		if stat.TableName != tableName {
			t.Errorf("expected table name %s, got %s", tableName, stat.TableName)
		}

		if stat.NTuples <= 0 {
			t.Errorf("expected positive tuple count, got %.2f", stat.NTuples)
		}

		if stat.TablePages <= 0 {
			t.Errorf("expected positive page count, got %.2f", stat.TablePages)
		}
	}
}

func BenchmarkCParser(b *testing.B) {
	queries := []string{
		"SELECT * FROM users",
		"SELECT id, name FROM users WHERE age > 18",
		"SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
		`WITH cte AS (SELECT * FROM users WHERE active = true) 
		 SELECT * FROM cte WHERE age > 25`,
		`SELECT 
			department,
			COUNT(*) as count,
			AVG(salary) as avg_salary,
			ROW_NUMBER() OVER (ORDER BY AVG(salary) DESC) as rank
		 FROM employees 
		 GROUP BY department 
		 HAVING COUNT(*) > 5`,
	}

	for i, query := range queries {
		b.Run(fmt.Sprintf("query_%d", i+1), func(b *testing.B) {
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				_, err := ParseSQLWithC(query)
				if err != nil {
					b.Fatalf("parse error: %v", err)
				}
			}
		})
	}
}

func BenchmarkCostEstimation(b *testing.B) {
	b.Run("seq_scan", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			CostSeqScan(1000, 100000)
		}
	})

	b.Run("index_scan", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			CostIndex(1000, 100000, 0.1)
		}
	})

	b.Run("hash_join", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			CostHashJoin(100, 200, 10000, 20000)
		}
	})

	b.Run("sort", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			CostSort(100000, 100)
		}
	})
}

func TestCParserErrorHandling(t *testing.T) {
	invalidQueries := []struct {
		name  string
		query string
	}{
		{
			name:  "syntax error",
			query: "SELECT * FORM users", // FORM instead of FROM
		},
		{
			name:  "unterminated string",
			query: "SELECT 'unterminated string FROM users",
		},
		{
			name:  "missing parenthesis",
			query: "SELECT * FROM users WHERE id IN (1, 2, 3",
		},
		{
			name:  "invalid operator",
			query: "SELECT * FROM users WHERE id @@ 'invalid'",
		},
	}

	for _, tt := range invalidQueries {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSQLWithC(tt.query)
			if err == nil {
				t.Errorf("expected error for invalid query: %s", tt.query)
			}
		})
	}
}

func TestCParserMemoryManagement(t *testing.T) {
	// Test that parser properly cleans up memory
	query := `SELECT * FROM users WHERE id IN (
		SELECT user_id FROM orders WHERE amount > 100
	)`

	// Parse the same query many times to test for memory leaks
	for i := 0; i < 10000; i++ {
		parser := NewCParser(query)
		if parser == nil {
			t.Fatal("failed to create parser")
		}

		_, err := parser.Parse()
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		parser.Close()
	}
}

func TestCParserConcurrency(t *testing.T) {
	query := "SELECT * FROM users WHERE age > 18"

	// Test concurrent parsing
	const numGoroutines = 100
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := ParseSQLWithC(query)
			errors <- err
		}()
	}

	// Check for errors
	for i := 0; i < numGoroutines; i++ {
		if err := <-errors; err != nil {
			t.Errorf("concurrent parsing error: %v", err)
		}
	}
}
