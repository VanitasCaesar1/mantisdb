// Integration tests for SQL JOIN operations
use mantisdb::error::Result as MantisResult;
use mantisdb::sql::parser::Parser;
use mantisdb::sql::ast::*;

#[test]
fn test_parse_inner_join() -> MantisResult<()> {
    let sql = "SELECT * FROM users INNER JOIN orders ON users.id = orders.user_id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert_eq!(select.joins[0].join_type, JoinType::Inner);
            assert_eq!(select.joins[0].table.name, "orders");
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_left_join() -> MantisResult<()> {
    let sql = "SELECT * FROM customers LEFT JOIN orders ON customers.id = orders.customer_id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert_eq!(select.joins[0].join_type, JoinType::Left);
            assert_eq!(select.joins[0].table.name, "orders");
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_right_join() -> MantisResult<()> {
    let sql = "SELECT * FROM orders RIGHT JOIN products ON orders.product_id = products.id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert_eq!(select.joins[0].join_type, JoinType::Right);
            assert_eq!(select.joins[0].table.name, "products");
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_multiple_joins() -> MantisResult<()> {
    let sql = "SELECT * FROM users 
               INNER JOIN orders ON users.id = orders.user_id
               LEFT JOIN products ON orders.product_id = products.id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 2);
            assert_eq!(select.joins[0].join_type, JoinType::Inner);
            assert_eq!(select.joins[0].table.name, "orders");
            assert_eq!(select.joins[1].join_type, JoinType::Left);
            assert_eq!(select.joins[1].table.name, "products");
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_join_with_where() -> MantisResult<()> {
    let sql = "SELECT * FROM users 
               INNER JOIN orders ON users.id = orders.user_id 
               WHERE users.active = true";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert!(select.where_clause.is_some());
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_join_with_table_alias() -> MantisResult<()> {
    let sql = "SELECT * FROM users u 
               INNER JOIN orders o ON u.id = o.user_id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.from.as_ref().unwrap().alias, Some("u".to_string()));
            assert_eq!(select.joins[0].table.alias, Some("o".to_string()));
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_left_outer_join() -> MantisResult<()> {
    let sql = "SELECT * FROM users LEFT OUTER JOIN orders ON users.id = orders.user_id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert_eq!(select.joins[0].join_type, JoinType::Left);
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_right_outer_join() -> MantisResult<()> {
    let sql = "SELECT * FROM orders RIGHT OUTER JOIN users ON orders.user_id = users.id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            assert_eq!(select.joins[0].join_type, JoinType::Right);
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_join_default_inner() -> MantisResult<()> {
    let sql = "SELECT * FROM users JOIN orders ON users.id = orders.user_id";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            // Default JOIN should be INNER
            assert_eq!(select.joins[0].join_type, JoinType::Inner);
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_join_with_complex_condition() -> MantisResult<()> {
    let sql = "SELECT * FROM users 
               JOIN orders ON users.id = orders.user_id AND orders.status = 'active'";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 1);
            // Verify join condition is parsed (should be AND expression)
            match &select.joins[0].condition {
                Expression::BinaryOp { op: BinaryOperator::And, .. } => {},
                _ => panic!("Expected AND in join condition"),
            }
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_three_way_join() -> MantisResult<()> {
    let sql = "SELECT * FROM users u
               INNER JOIN orders o ON u.id = o.user_id
               INNER JOIN products p ON o.product_id = p.id
               WHERE u.active = true";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert_eq!(select.joins.len(), 2);
            assert_eq!(select.joins[0].table.name, "orders");
            assert_eq!(select.joins[1].table.name, "products");
            assert!(select.where_clause.is_some());
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_parse_subquery_in_where() -> MantisResult<()> {
    let sql = "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)";
    let mut parser = Parser::new(sql)?;
    let statement = parser.parse()?;
    
    match statement {
        Statement::Select(select) => {
            assert!(select.where_clause.is_some());
            // Verify subquery is parsed
            match select.where_clause.as_ref().unwrap() {
                Expression::InList { list, .. } => {
                    // First item in list should be a subquery
                    if let Some(Expression::Subquery(_)) = list.first() {
                        // Success
                    } else {
                        panic!("Expected subquery in IN list");
                    }
                }
                _ => panic!("Expected IN expression with subquery"),
            }
        }
        _ => panic!("Expected SELECT statement"),
    }
    
    Ok(())
}

#[test]
fn test_join_ast_structure() {
    // Test the AST structure directly
    let join = Join {
        join_type: JoinType::Inner,
        table: TableReference {
            name: "orders".to_string(),
            alias: Some("o".to_string()),
        },
        condition: Expression::BinaryOp {
            left: Box::new(Expression::Identifier("users.id".to_string())),
            op: BinaryOperator::Equal,
            right: Box::new(Expression::Identifier("orders.user_id".to_string())),
        },
    };
    
    assert_eq!(join.table.name, "orders");
    assert_eq!(join.table.alias, Some("o".to_string()));
    assert_eq!(join.join_type, JoinType::Inner);
}

#[test]
fn test_all_join_types() {
    // Verify all join types are defined
    let types = vec![
        JoinType::Inner,
        JoinType::Left,
        JoinType::Right,
        JoinType::Full,
        JoinType::Cross,
    ];
    
    assert_eq!(types.len(), 5);
}
