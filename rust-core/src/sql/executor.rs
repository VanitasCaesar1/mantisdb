// Query Executor - Execute optimized query plans
use super::optimizer::*;
use super::types::*;
use crate::error::MantisError;

pub struct QueryExecutor {
    // Storage engine interface would go here
}

impl QueryExecutor {
    pub fn new() -> Self {
        QueryExecutor {}
    }
    
    pub fn execute(&self, plan: &QueryPlan) -> Result<QueryResult, MantisError> {
        self.execute_node(&plan.root)
    }
    
    fn execute_node(&self, node: &PlanNode) -> Result<QueryResult, MantisError> {
        match node {
            PlanNode::TableScan { table, filter } => {
                self.execute_table_scan(table, filter.as_ref())
            }
            PlanNode::IndexScan { table, index, filter } => {
                self.execute_index_scan(table, index, filter.as_ref())
            }
            PlanNode::NestedLoopJoin { left, right, condition } => {
                self.execute_nested_loop_join(left, right, condition)
            }
            PlanNode::HashJoin { left, right, condition } => {
                self.execute_hash_join(left, right, condition)
            }
            PlanNode::Sort { input, order_by } => {
                self.execute_sort(input, order_by)
            }
            PlanNode::Limit { input, limit, offset } => {
                self.execute_limit(input, *limit, *offset)
            }
        }
    }
    
    fn execute_table_scan(
        &self,
        _table: &str,
        _filter: Option<&super::ast::Expression>,
    ) -> Result<QueryResult, MantisError> {
        // TODO: Implement table scan
        Ok(QueryResult {
            columns: vec!["id".to_string(), "name".to_string()],
            rows: vec![],
            rows_affected: 0,
        })
    }
    
    fn execute_index_scan(
        &self,
        _table: &str,
        _index: &str,
        _filter: Option<&super::ast::Expression>,
    ) -> Result<QueryResult, MantisError> {
        // TODO: Implement index scan
        Ok(QueryResult {
            columns: vec![],
            rows: vec![],
            rows_affected: 0,
        })
    }
    
    fn execute_nested_loop_join(
        &self,
        left: &PlanNode,
        right: &PlanNode,
        condition: &super::ast::Expression,
    ) -> Result<QueryResult, MantisError> {
        // Execute left and right sides
        let left_result = self.execute_node(left)?;
        let right_result = self.execute_node(right)?;
        
        // Combine column names
        let mut columns = left_result.columns.clone();
        columns.extend(right_result.columns.clone());
        
        let mut joined_rows = Vec::new();
        
        // Nested loop join algorithm
        for left_row in &left_result.rows {
            for right_row in &right_result.rows {
                // Combine rows
                let mut combined_row = left_row.clone();
                combined_row.extend(right_row.clone());
                
                // Evaluate join condition
                if self.evaluate_join_condition(condition, &combined_row, &columns)? {
                    joined_rows.push(combined_row);
                }
            }
        }
        
        Ok(QueryResult {
            columns,
            rows: joined_rows,
            rows_affected: 0,
        })
    }
    
    fn execute_hash_join(
        &self,
        left: &PlanNode,
        right: &PlanNode,
        condition: &super::ast::Expression,
    ) -> Result<QueryResult, MantisError> {
        use std::collections::HashMap;
        
        // Execute left and right sides
        let left_result = self.execute_node(left)?;
        let right_result = self.execute_node(right)?;
        
        // Combine column names
        let mut columns = left_result.columns.clone();
        columns.extend(right_result.columns.clone());
        
        // Build phase: create hash table from smaller table (left)
        let mut hash_table: HashMap<String, Vec<Vec<SqlValue>>> = HashMap::new();
        
        for row in &left_result.rows {
            // Use first column as hash key (simplified)
            if let Some(key_value) = row.first() {
                let key = format!("{:?}", key_value);
                hash_table.entry(key).or_insert_with(Vec::new).push(row.clone());
            }
        }
        
        // Probe phase: match right table rows
        let mut joined_rows = Vec::new();
        
        for right_row in &right_result.rows {
            if let Some(key_value) = right_row.first() {
                let key = format!("{:?}", key_value);
                
                if let Some(matching_left_rows) = hash_table.get(&key) {
                    for left_row in matching_left_rows {
                        let mut combined_row = left_row.clone();
                        combined_row.extend(right_row.clone());
                        
                        // Evaluate join condition
                        if self.evaluate_join_condition(condition, &combined_row, &columns)? {
                            joined_rows.push(combined_row);
                        }
                    }
                }
            }
        }
        
        Ok(QueryResult {
            columns,
            rows: joined_rows,
            rows_affected: 0,
        })
    }
    
    fn execute_sort(
        &self,
        input: &PlanNode,
        _order_by: &[super::ast::OrderByItem],
    ) -> Result<QueryResult, MantisError> {
        let result = self.execute_node(input)?;
        // TODO: Implement sorting
        Ok(result)
    }
    
    fn execute_limit(
        &self,
        input: &PlanNode,
        limit: u64,
        offset: u64,
    ) -> Result<QueryResult, MantisError> {
        let mut result = self.execute_node(input)?;
        
        let start = offset as usize;
        let end = (offset + limit) as usize;
        
        if start < result.rows.len() {
            result.rows = result.rows[start..end.min(result.rows.len())].to_vec();
        } else {
            result.rows.clear();
        }
        
        Ok(result)
    }
}

    /// Evaluate join condition on a combined row
    fn evaluate_join_condition(
        &self,
        _condition: &super::ast::Expression,
        _row: &[SqlValue],
        _columns: &[String],
    ) -> Result<bool, MantisError> {
        // Simplified: always return true
        // In production, this would evaluate the expression against the row
        // TODO: Implement full expression evaluation with column lookups
        Ok(true)
    }
}

impl Default for QueryExecutor {
    fn default() -> Self {
        Self::new()
    }
}
