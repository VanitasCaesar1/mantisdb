//! Query Builder - Type-safe SQL query construction
//!
//! Provides a fluent API for building SQL queries with compile-time safety

use crate::error::{Error, Result};
use std::fmt;

/// Query builder for type-safe SQL construction
pub struct QueryBuilder {
    select: Vec<String>,
    from: Option<String>,
    joins: Vec<JoinClause>,
    where_clauses: Vec<WhereClause>,
    group_by: Vec<String>,
    having: Option<String>,
    order_by: Vec<OrderByClause>,
    limit: Option<usize>,
    offset: Option<usize>,
}

impl QueryBuilder {
    pub fn new() -> Self {
        Self {
            select: Vec::new(),
            from: None,
            joins: Vec::new(),
            where_clauses: Vec::new(),
            group_by: Vec::new(),
            having: None,
            order_by: Vec::new(),
            limit: None,
            offset: None,
        }
    }
    
    /// Start building a SELECT query
    pub fn from(table: impl Into<String>) -> Self {
        Self {
            from: Some(table.into()),
            ..Self::new()
        }
    }
    
    /// Select specific columns
    pub fn select(mut self, columns: &[&str]) -> Self {
        self.select = columns.iter().map(|s| s.to_string()).collect();
        self
    }
    
    /// Select all columns
    pub fn select_all(mut self) -> Self {
        self.select = vec!["*".to_string()];
        self
    }
    
    /// Add WHERE clause with equality
    pub fn where_eq(mut self, column: &str, value: impl fmt::Display) -> Self {
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator: Operator::Equal,
            value: value.to_string(),
            connector: Connector::And,
        });
        self
    }
    
    /// Add WHERE clause with greater than
    pub fn where_gt(mut self, column: &str, value: impl fmt::Display) -> Self {
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator: Operator::GreaterThan,
            value: value.to_string(),
            connector: Connector::And,
        });
        self
    }
    
    /// Add WHERE clause with less than
    pub fn where_lt(mut self, column: &str, value: impl fmt::Display) -> Self {
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator: Operator::LessThan,
            value: value.to_string(),
            connector: Connector::And,
        });
        self
    }
    
    /// Add WHERE clause with LIKE
    pub fn where_like(mut self, column: &str, pattern: &str) -> Self {
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator: Operator::Like,
            value: pattern.to_string(),
            connector: Connector::And,
        });
        self
    }
    
    /// Add WHERE clause with IN
    pub fn where_in(mut self, column: &str, values: Vec<String>) -> Self {
        let value_list = values.join(", ");
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator: Operator::In,
            value: format!("({})", value_list),
            connector: Connector::And,
        });
        self
    }
    
    /// Add OR WHERE clause
    pub fn or_where(mut self, column: &str, operator: Operator, value: impl fmt::Display) -> Self {
        self.where_clauses.push(WhereClause {
            column: column.to_string(),
            operator,
            value: value.to_string(),
            connector: Connector::Or,
        });
        self
    }
    
    /// Add INNER JOIN
    pub fn join(mut self, table: &str, on_left: &str, on_right: &str) -> Self {
        self.joins.push(JoinClause {
            join_type: JoinType::Inner,
            table: table.to_string(),
            on_left: on_left.to_string(),
            on_right: on_right.to_string(),
        });
        self
    }
    
    /// Add LEFT JOIN
    pub fn left_join(mut self, table: &str, on_left: &str, on_right: &str) -> Self {
        self.joins.push(JoinClause {
            join_type: JoinType::Left,
            table: table.to_string(),
            on_left: on_left.to_string(),
            on_right: on_right.to_string(),
        });
        self
    }
    
    /// Add ORDER BY ascending
    pub fn order_by(mut self, column: &str) -> Self {
        self.order_by.push(OrderByClause {
            column: column.to_string(),
            direction: OrderDirection::Asc,
        });
        self
    }
    
    /// Add ORDER BY descending
    pub fn order_by_desc(mut self, column: &str) -> Self {
        self.order_by.push(OrderByClause {
            column: column.to_string(),
            direction: OrderDirection::Desc,
        });
        self
    }
    
    /// Add GROUP BY
    pub fn group_by(mut self, columns: &[&str]) -> Self {
        self.group_by = columns.iter().map(|s| s.to_string()).collect();
        self
    }
    
    /// Add HAVING clause
    pub fn having(mut self, condition: &str) -> Self {
        self.having = Some(condition.to_string());
        self
    }
    
    /// Set LIMIT
    pub fn limit(mut self, limit: usize) -> Self {
        self.limit = Some(limit);
        self
    }
    
    /// Set OFFSET
    pub fn offset(mut self, offset: usize) -> Self {
        self.offset = Some(offset);
        self
    }
    
    /// Build the SQL query string
    pub fn build(&self) -> Result<String> {
        let mut query = String::from("SELECT ");
        
        // SELECT clause
        if self.select.is_empty() {
            query.push_str("*");
        } else {
            query.push_str(&self.select.join(", "));
        }
        
        // FROM clause
        if let Some(ref table) = self.from {
            query.push_str(&format!(" FROM {}", table));
        } else {
            return Err(Error::ValidationError("FROM clause is required".to_string()));
        }
        
        // JOIN clauses
        for join in &self.joins {
            query.push_str(&format!(
                " {} JOIN {} ON {} = {}",
                join.join_type,
                join.table,
                join.on_left,
                join.on_right
            ));
        }
        
        // WHERE clauses
        if !self.where_clauses.is_empty() {
            query.push_str(" WHERE ");
            for (i, clause) in self.where_clauses.iter().enumerate() {
                if i > 0 {
                    query.push_str(&format!(" {} ", clause.connector));
                }
                query.push_str(&format!(
                    "{} {} {}",
                    clause.column,
                    clause.operator,
                    if clause.operator == Operator::Like {
                        format!("'{}'", clause.value)
                    } else if clause.operator == Operator::In {
                        clause.value.clone()
                    } else {
                        format!("'{}'", clause.value)
                    }
                ));
            }
        }
        
        // GROUP BY clause
        if !self.group_by.is_empty() {
            query.push_str(&format!(" GROUP BY {}", self.group_by.join(", ")));
        }
        
        // HAVING clause
        if let Some(ref having) = self.having {
            query.push_str(&format!(" HAVING {}", having));
        }
        
        // ORDER BY clause
        if !self.order_by.is_empty() {
            query.push_str(" ORDER BY ");
            let order_parts: Vec<String> = self.order_by
                .iter()
                .map(|o| format!("{} {}", o.column, o.direction))
                .collect();
            query.push_str(&order_parts.join(", "));
        }
        
        // LIMIT clause
        if let Some(limit) = self.limit {
            query.push_str(&format!(" LIMIT {}", limit));
        }
        
        // OFFSET clause
        if let Some(offset) = self.offset {
            query.push_str(&format!(" OFFSET {}", offset));
        }
        
        Ok(query)
    }
    
    /// Execute the query (placeholder)
    pub fn execute(&self) -> Result<Vec<serde_json::Value>> {
        let sql = self.build()?;
        println!("Executing: {}", sql);
        // Would call actual SQL executor here
        Ok(vec![])
    }
}

impl Default for QueryBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone)]
struct WhereClause {
    column: String,
    operator: Operator,
    value: String,
    connector: Connector,
}

#[derive(Debug, Clone)]
struct JoinClause {
    join_type: JoinType,
    table: String,
    on_left: String,
    on_right: String,
}

#[derive(Debug, Clone)]
struct OrderByClause {
    column: String,
    direction: OrderDirection,
}

#[derive(Debug, Clone, PartialEq)]
pub enum Operator {
    Equal,
    NotEqual,
    GreaterThan,
    LessThan,
    GreaterOrEqual,
    LessOrEqual,
    Like,
    In,
}

impl fmt::Display for Operator {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            Operator::Equal => write!(f, "="),
            Operator::NotEqual => write!(f, "!="),
            Operator::GreaterThan => write!(f, ">"),
            Operator::LessThan => write!(f, "<"),
            Operator::GreaterOrEqual => write!(f, ">="),
            Operator::LessOrEqual => write!(f, "<="),
            Operator::Like => write!(f, "LIKE"),
            Operator::In => write!(f, "IN"),
        }
    }
}

#[derive(Debug, Clone)]
enum Connector {
    And,
    Or,
}

impl fmt::Display for Connector {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            Connector::And => write!(f, "AND"),
            Connector::Or => write!(f, "OR"),
        }
    }
}

#[derive(Debug, Clone)]
enum JoinType {
    Inner,
    Left,
    Right,
    Full,
}

impl fmt::Display for JoinType {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            JoinType::Inner => write!(f, "INNER"),
            JoinType::Left => write!(f, "LEFT"),
            JoinType::Right => write!(f, "RIGHT"),
            JoinType::Full => write!(f, "FULL"),
        }
    }
}

#[derive(Debug, Clone)]
enum OrderDirection {
    Asc,
    Desc,
}

impl fmt::Display for OrderDirection {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            OrderDirection::Asc => write!(f, "ASC"),
            OrderDirection::Desc => write!(f, "DESC"),
        }
    }
}

/// Convenient table-specific query builder
pub struct Table {
    name: String,
}

impl Table {
    pub fn new(name: impl Into<String>) -> Self {
        Self {
            name: name.into(),
        }
    }
    
    /// Start SELECT query on this table
    pub fn select(&self, columns: &[&str]) -> QueryBuilder {
        QueryBuilder::from(&self.name).select(columns)
    }
    
    /// Select all from this table
    pub fn select_all(&self) -> QueryBuilder {
        QueryBuilder::from(&self.name).select_all()
    }
    
    /// Find by ID (assumes 'id' column)
    pub fn find(&self, id: impl fmt::Display) -> QueryBuilder {
        QueryBuilder::from(&self.name)
            .select_all()
            .where_eq("id", id)
            .limit(1)
    }
    
    /// Find all with condition
    pub fn find_all(&self) -> QueryBuilder {
        QueryBuilder::from(&self.name).select_all()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_simple_select() {
        let query = QueryBuilder::from("users")
            .select_all()
            .build()
            .unwrap();
        
        assert_eq!(query, "SELECT * FROM users");
    }
    
    #[test]
    fn test_select_with_where() {
        let query = QueryBuilder::from("users")
            .select(&["id", "name", "email"])
            .where_eq("active", "true")
            .where_gt("age", 18)
            .build()
            .unwrap();
        
        assert_eq!(
            query,
            "SELECT id, name, email FROM users WHERE active = 'true' AND age > '18'"
        );
    }
    
    #[test]
    fn test_join_query() {
        let query = QueryBuilder::from("users")
            .select(&["users.name", "orders.total"])
            .join("orders", "users.id", "orders.user_id")
            .where_eq("users.active", "true")
            .build()
            .unwrap();
        
        assert!(query.contains("INNER JOIN orders"));
        assert!(query.contains("ON users.id = orders.user_id"));
    }
    
    #[test]
    fn test_order_and_limit() {
        let query = QueryBuilder::from("posts")
            .select_all()
            .order_by_desc("created_at")
            .limit(10)
            .offset(20)
            .build()
            .unwrap();
        
        assert!(query.contains("ORDER BY created_at DESC"));
        assert!(query.contains("LIMIT 10"));
        assert!(query.contains("OFFSET 20"));
    }
    
    #[test]
    fn test_group_by_having() {
        let query = QueryBuilder::from("orders")
            .select(&["user_id", "COUNT(*) as total"])
            .group_by(&["user_id"])
            .having("COUNT(*) > 5")
            .build()
            .unwrap();
        
        assert!(query.contains("GROUP BY user_id"));
        assert!(query.contains("HAVING COUNT(*) > 5"));
    }
    
    #[test]
    fn test_table_helper() {
        let users = Table::new("users");
        
        let query = users
            .select(&["name", "email"])
            .where_eq("active", "true")
            .build()
            .unwrap();
        
        assert!(query.contains("SELECT name, email"));
        assert!(query.contains("FROM users"));
    }
    
    #[test]
    fn test_find_by_id() {
        let users = Table::new("users");
        let query = users.find(123).build().unwrap();
        
        assert!(query.contains("WHERE id = '123'"));
        assert!(query.contains("LIMIT 1"));
    }
}
