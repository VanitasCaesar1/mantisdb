package sql

import (
	"fmt"
	"testing"
)

func TestQueryOptimizer(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Test basic SELECT optimization
	t.Run("BasicSelect", func(t *testing.T) {
		stmt := &SelectStatement{
			Fields: []SelectField{
				{Expression: &IdentifierExpression{Name: "id"}},
				{Expression: &IdentifierExpression{Name: "name"}},
			},
			From: []TableReference{
				{Name: "users"},
			},
		}

		plan, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			t.Fatalf("Failed to optimize query: %v", err)
		}

		if plan == nil {
			t.Fatal("Expected non-nil plan")
		}

		if plan.Type != PlanTypeSeqScan {
			t.Errorf("Expected sequential scan, got %v", plan.Type)
		}
	})

	// Test SELECT with WHERE clause
	t.Run("SelectWithWhere", func(t *testing.T) {
		stmt := &SelectStatement{
			Fields: []SelectField{
				{Expression: &IdentifierExpression{Name: "name"}},
			},
			From: []TableReference{
				{Name: "users"},
			},
			Where: &BinaryExpression{
				Left:     &IdentifierExpression{Name: "id"},
				Operator: OpEqual,
				Right:    &LiteralExpression{Value: 1},
			},
		}

		plan, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			t.Fatalf("Failed to optimize query: %v", err)
		}

		if len(plan.Qual) == 0 {
			t.Error("Expected WHERE clause in plan qualifiers")
		}
	})

	// Test JOIN optimization
	t.Run("JoinOptimization", func(t *testing.T) {
		stmt := &SelectStatement{
			Fields: []SelectField{
				{Expression: &IdentifierExpression{Name: "u.name"}},
				{Expression: &IdentifierExpression{Name: "p.title"}},
			},
			From: []TableReference{
				{Name: "users", Alias: "u"},
				{Name: "posts", Alias: "p"},
			},
			Where: &BinaryExpression{
				Left:     &IdentifierExpression{Name: "u.id"},
				Operator: OpEqual,
				Right:    &IdentifierExpression{Name: "p.user_id"},
			},
		}

		plan, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			t.Fatalf("Failed to optimize join query: %v", err)
		}

		// Should generate a join plan
		if plan.Type != PlanTypeNestLoop && plan.Type != PlanTypeHashJoin && plan.Type != PlanTypeMergeJoin {
			t.Errorf("Expected join plan, got %v", plan.Type)
		}
	})

	// Test ORDER BY optimization
	t.Run("OrderByOptimization", func(t *testing.T) {
		stmt := &SelectStatement{
			Fields: []SelectField{
				{Expression: &IdentifierExpression{Name: "name"}},
			},
			From: []TableReference{
				{Name: "users"},
			},
			OrderBy: []OrderByClause{
				{
					Expression: &IdentifierExpression{Name: "name"},
					Direction:  Ascending,
				},
			},
		}

		plan, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			t.Fatalf("Failed to optimize ORDER BY query: %v", err)
		}

		// Should have a sort node somewhere in the plan
		if !hasSortNode(plan) {
			t.Error("Expected sort node in plan for ORDER BY")
		}
	})

	// Test LIMIT optimization
	t.Run("LimitOptimization", func(t *testing.T) {
		stmt := &SelectStatement{
			Fields: []SelectField{
				{Expression: &IdentifierExpression{Name: "name"}},
			},
			From: []TableReference{
				{Name: "users"},
			},
			Limit: &LimitClause{Count: &LiteralExpression{Value: 10}},
		}

		plan, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			t.Fatalf("Failed to optimize LIMIT query: %v", err)
		}

		// Should have a limit node
		if !hasLimitNode(plan) {
			t.Error("Expected limit node in plan for LIMIT")
		}
	})
}

func TestPlanCache(t *testing.T) {
	cache := NewPlanCache(2)

	plan1 := &QueryPlan{Type: PlanTypeSeqScan, TotalCost: 100}
	plan2 := &QueryPlan{Type: PlanTypeIndexScan, TotalCost: 50}
	plan3 := &QueryPlan{Type: PlanTypeHashJoin, TotalCost: 200}

	// Test cache put and get
	cache.Put("query1", plan1)
	cache.Put("query2", plan2)

	if retrieved, found := cache.Get("query1"); !found || retrieved != plan1 {
		t.Error("Failed to retrieve cached plan")
	}

	// Test cache eviction
	cache.Put("query3", plan3) // Should evict query1 (LRU)

	if _, found := cache.Get("query1"); found {
		t.Error("Expected query1 to be evicted")
	}

	if _, found := cache.Get("query2"); !found {
		t.Error("Expected query2 to still be cached")
	}

	// Test cache statistics
	hits, misses, hitRatio := cache.GetStats()
	if hits == 0 {
		t.Error("Expected some cache hits")
	}
	if hitRatio <= 0 {
		t.Error("Expected positive hit ratio")
	}
	t.Logf("Cache stats: hits=%d, misses=%d, ratio=%.2f", hits, misses, hitRatio)
}

func TestOptimizerCostEstimation(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Create test table statistics
	stats := &TableStatistics{
		TableName:   "test_table",
		RowCount:    10000,
		PageCount:   1000,
		AvgRowWidth: 100,
	}

	// Test sequential scan cost
	plan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		TableName: "test_table",
		PlanRows:  stats.RowCount,
		PlanWidth: int(stats.AvgRowWidth),
	}

	optimizer.costSeqScan(plan, stats)

	if plan.TotalCost <= 0 {
		t.Error("Expected positive cost for sequential scan")
	}

	t.Logf("Sequential scan cost: startup=%.2f, total=%.2f", plan.StartupCost, plan.TotalCost)

	// Test join cost estimation
	leftPlan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		PlanRows:  1000,
		PlanWidth: 50,
		TotalCost: 100,
	}

	rightPlan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		PlanRows:  500,
		PlanWidth: 75,
		TotalCost: 50,
	}

	joinPlan := &QueryPlan{
		Type:      PlanTypeHashJoin,
		LeftTree:  leftPlan,
		RightTree: rightPlan,
		JoinType:  InnerJoin,
	}

	optimizer.costHashJoin(joinPlan)

	if joinPlan.TotalCost <= leftPlan.TotalCost+rightPlan.TotalCost {
		t.Error("Expected join cost to be higher than sum of input costs")
	}

	t.Logf("Hash join cost: startup=%.2f, total=%.2f, rows=%.0f",
		joinPlan.StartupCost, joinPlan.TotalCost, joinPlan.PlanRows)
}

func TestJoinSelectivityEstimation(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Test equality join selectivity
	joinClause := &BinaryExpression{
		Left:     &IdentifierExpression{Name: "t1.id"},
		Operator: OpEqual,
		Right:    &IdentifierExpression{Name: "t2.id"},
	}

	selectivity := optimizer.estimateJoinClauseSelectivity(joinClause)
	if selectivity <= 0 || selectivity > 1 {
		t.Errorf("Invalid selectivity: %f", selectivity)
	}

	t.Logf("Equality join selectivity: %.4f", selectivity)

	// Test multiple join clauses
	joinClauses := []Expression{
		joinClause,
		&BinaryExpression{
			Left:     &IdentifierExpression{Name: "t1.status"},
			Operator: OpEqual,
			Right:    &IdentifierExpression{Name: "t2.status"},
		},
	}

	multiSelectivity := optimizer.estimateJoinSelectivity(joinClauses)
	if multiSelectivity >= selectivity {
		t.Error("Expected lower selectivity for multiple join clauses")
	}

	t.Logf("Multi-clause join selectivity: %.4f", multiSelectivity)
}

func TestDynamicProgrammingJoinOptimization(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Create test tables
	tables := []TableReference{
		{Name: "table1"},
		{Name: "table2"},
		{Name: "table3"},
	}

	// Add some mock statistics
	for _, table := range tables {
		stats := &TableStatistics{
			TableName:   table.Name,
			RowCount:    1000,
			PageCount:   100,
			AvgRowWidth: 50,
		}
		optimizer.stats.UpdateTableStats(stats)
	}

	plan, err := optimizer.buildJoinPlanDP(tables)
	if err != nil {
		t.Fatalf("Failed to build join plan: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected non-nil join plan")
	}

	// Verify the plan structure
	if !isJoinPlan(plan) {
		t.Error("Expected join plan structure")
	}

	t.Logf("DP join plan cost: %.2f", plan.TotalCost)
}

func TestQueryRewriter(t *testing.T) {
	rewriter := NewQueryRewriter()

	stmt := &SelectStatement{
		Fields: []SelectField{
			{Expression: &IdentifierExpression{Name: "name"}},
		},
		From: []TableReference{
			{Name: "users"},
		},
		Where: &BinaryExpression{
			Left:     &LiteralExpression{Value: 1},
			Operator: OpEqual,
			Right:    &LiteralExpression{Value: 1},
		},
	}

	rewritten := rewriter.Rewrite(stmt)
	if rewritten == nil {
		t.Fatal("Expected non-nil rewritten statement")
	}

	// In a real implementation, constant folding would simplify the WHERE clause
	t.Logf("Rewriter processed statement successfully")
}

func TestParallelScanOptimization(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Large table that should benefit from parallel scan
	stats := &TableStatistics{
		TableName:   "large_table",
		RowCount:    1000000,
		PageCount:   100000,
		AvgRowWidth: 100,
	}

	plan, err := optimizer.chooseScanPlan("large_table", stats, nil)
	if err != nil {
		t.Fatalf("Failed to choose scan plan: %v", err)
	}

	// Should consider parallel scan for large tables
	if plan.Type != PlanTypeParallelSeqScan && plan.Type != PlanTypeSeqScan {
		t.Errorf("Expected parallel or sequential scan, got %v", plan.Type)
	}

	if plan.Type == PlanTypeParallelSeqScan {
		if plan.Workers <= 1 {
			t.Error("Expected multiple workers for parallel scan")
		}
		t.Logf("Parallel scan with %d workers", plan.Workers)
	}
}

func TestMemoryConstrainedHashJoin(t *testing.T) {
	optimizer := NewQueryOptimizer()

	// Set small work memory to test spilling behavior
	optimizer.config.WorkMem = 1024 // 1MB

	leftPlan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		PlanRows:  100000,
		PlanWidth: 100,
		TotalCost: 1000,
	}

	rightPlan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		PlanRows:  50000,
		PlanWidth: 150,
		TotalCost: 500,
	}

	joinPlan := &QueryPlan{
		Type:      PlanTypeHashJoin,
		LeftTree:  leftPlan,
		RightTree: rightPlan,
		JoinType:  InnerJoin,
	}

	optimizer.costHashJoin(joinPlan)

	// Should account for spilling cost
	expectedMinCost := leftPlan.TotalCost + rightPlan.TotalCost
	if joinPlan.TotalCost <= expectedMinCost {
		t.Error("Expected higher cost due to memory constraints")
	}

	t.Logf("Memory-constrained hash join cost: %.2f", joinPlan.TotalCost)
}

// Helper functions for tests

func hasSortNode(plan *QueryPlan) bool {
	if plan == nil {
		return false
	}
	if plan.Type == PlanTypeSort {
		return true
	}
	return hasSortNode(plan.LeftTree) || hasSortNode(plan.RightTree)
}

func hasLimitNode(plan *QueryPlan) bool {
	if plan == nil {
		return false
	}
	if plan.Type == PlanTypeLimit {
		return true
	}
	return hasLimitNode(plan.LeftTree) || hasLimitNode(plan.RightTree)
}

func isJoinPlan(plan *QueryPlan) bool {
	if plan == nil {
		return false
	}
	return plan.Type == PlanTypeNestLoop ||
		plan.Type == PlanTypeHashJoin ||
		plan.Type == PlanTypeMergeJoin
}

func BenchmarkQueryOptimization(b *testing.B) {
	optimizer := NewQueryOptimizer()

	stmt := &SelectStatement{
		Fields: []SelectField{
			{Expression: &IdentifierExpression{Name: "u.name"}},
			{Expression: &IdentifierExpression{Name: "p.title"}},
		},
		From: []TableReference{
			{Name: "users", Alias: "u"},
			{Name: "posts", Alias: "p"},
		},
		Where: &BinaryExpression{
			Left:     &IdentifierExpression{Name: "u.id"},
			Operator: OpEqual,
			Right:    &IdentifierExpression{Name: "p.user_id"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := optimizer.OptimizeQuery(stmt)
		if err != nil {
			b.Fatalf("Optimization failed: %v", err)
		}
	}
}

func BenchmarkPlanCache(b *testing.B) {
	cache := NewPlanCache(1000)
	plan := &QueryPlan{Type: PlanTypeSeqScan, TotalCost: 100}

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		cache.Put(fmt.Sprintf("query%d", i), plan)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("query%d", i%100)
		cache.Get(key)
	}
}
