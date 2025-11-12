// High-Performance SQL Parser
use super::ast::*;
use super::lexer::{Lexer, Token};
use super::types::SqlValue;
use crate::error::MantisError;

pub struct Parser {
    tokens: Vec<Token>,
    position: usize,
}

impl Parser {
    pub fn new(sql: &str) -> Result<Self, MantisError> {
        let mut lexer = Lexer::new(sql);
        let tokens = lexer.tokenize();
        
        Ok(Parser {
            tokens,
            position: 0,
        })
    }
    
    fn current_token(&self) -> &Token {
        self.tokens.get(self.position).unwrap_or(&Token::Eof)
    }
    
    fn peek_token(&self, offset: usize) -> &Token {
        self.tokens.get(self.position + offset).unwrap_or(&Token::Eof)
    }
    
    fn advance(&mut self) {
        if self.position < self.tokens.len() {
            self.position += 1;
        }
    }
    
    fn expect(&mut self, expected: Token) -> Result<(), MantisError> {
        if self.current_token() == &expected {
            self.advance();
            Ok(())
        } else {
            Err(MantisError::ParseError(format!(
                "Expected {:?}, found {:?}",
                expected,
                self.current_token()
            )))
        }
    }
    
    pub fn parse(&mut self) -> Result<Statement, MantisError> {
        match self.current_token() {
            Token::Select => self.parse_select(),
            Token::Insert => self.parse_insert(),
            Token::Update => self.parse_update(),
            Token::Delete => self.parse_delete(),
            Token::Create => self.parse_create(),
            Token::Drop => self.parse_drop(),
            _ => Err(MantisError::ParseError(format!(
                "Unexpected token: {:?}",
                self.current_token()
            ))),
        }
    }
    
    fn parse_select(&mut self) -> Result<Statement, MantisError> {
        self.expect(Token::Select)?;
        
        let distinct = if self.current_token() == &Token::Distinct {
            self.advance();
            true
        } else {
            false
        };
        
        let columns = self.parse_select_items()?;
        
        let from = if self.current_token() == &Token::From {
            self.advance();
            Some(self.parse_table_reference()?)
        } else {
            None
        };
        
        // Parse JOINs
        let mut joins = Vec::new();
        while self.is_join_keyword() {
            joins.push(self.parse_join()?);
        }
        
        let where_clause = if self.current_token() == &Token::Where {
            self.advance();
            Some(self.parse_expression()?)
        } else {
            None
        };
        
        let group_by = if self.current_token() == &Token::Group {
            self.advance();
            self.expect(Token::By)?;
            self.parse_expression_list()?
        } else {
            Vec::new()
        };
        
        let having = if self.current_token() == &Token::Having {
            self.advance();
            Some(self.parse_expression()?)
        } else {
            None
        };
        
        let order_by = if self.current_token() == &Token::Order {
            self.advance();
            self.expect(Token::By)?;
            self.parse_order_by_items()?
        } else {
            Vec::new()
        };
        
        let limit = if self.current_token() == &Token::Limit {
            self.advance();
            if let Token::IntegerLiteral(n) = self.current_token() {
                let limit_val = *n as u64;
                self.advance();
                Some(limit_val)
            } else {
                return Err(MantisError::ParseError("Expected integer after LIMIT".to_string()));
            }
        } else {
            None
        };
        
        let offset = if self.current_token() == &Token::Offset {
            self.advance();
            if let Token::IntegerLiteral(n) = self.current_token() {
                let offset_val = *n as u64;
                self.advance();
                Some(offset_val)
            } else {
                return Err(MantisError::ParseError("Expected integer after OFFSET".to_string()));
            }
        } else {
            None
        };
        
        Ok(Statement::Select(SelectStatement {
            distinct,
            columns,
            from,
            joins,
            where_clause,
            group_by,
            having,
            order_by,
            limit,
            offset,
        }))
    }
    
    fn parse_select_items(&mut self) -> Result<Vec<SelectItem>, MantisError> {
        let mut items = Vec::new();
        
        loop {
            if self.current_token() == &Token::Star {
                self.advance();
                items.push(SelectItem::Wildcard);
            } else {
                let expr = self.parse_expression()?;
                let alias = if self.current_token() == &Token::As {
                    self.advance();
                    if let Token::Identifier(name) = self.current_token() {
                        let alias_name = name.clone();
                        self.advance();
                        Some(alias_name)
                    } else {
                        None
                    }
                } else {
                    None
                };
                items.push(SelectItem::Expression { expr, alias });
            }
            
            if self.current_token() == &Token::Comma {
                self.advance();
            } else {
                break;
            }
        }
        
        Ok(items)
    }
    
    fn parse_table_reference(&mut self) -> Result<TableReference, MantisError> {
        if let Token::Identifier(name) = self.current_token() {
            let table_name = name.clone();
            self.advance();
            
            let alias = if self.current_token() == &Token::As {
                self.advance();
                if let Token::Identifier(alias_name) = self.current_token() {
                    let alias = alias_name.clone();
                    self.advance();
                    Some(alias)
                } else {
                    None
                }
            } else {
                None
            };
            
            Ok(TableReference {
                name: table_name,
                alias,
            })
        } else {
            Err(MantisError::ParseError("Expected table name".to_string()))
        }
    }
    
    fn parse_expression(&mut self) -> Result<Expression, MantisError> {
        self.parse_or_expression()
    }
    
    fn parse_or_expression(&mut self) -> Result<Expression, MantisError> {
        let mut left = self.parse_and_expression()?;
        
        while self.current_token() == &Token::Or {
            self.advance();
            let right = self.parse_and_expression()?;
            left = Expression::BinaryOp {
                left: Box::new(left),
                op: BinaryOperator::Or,
                right: Box::new(right),
            };
        }
        
        Ok(left)
    }
    
    fn parse_and_expression(&mut self) -> Result<Expression, MantisError> {
        let mut left = self.parse_comparison_expression()?;
        
        while self.current_token() == &Token::And {
            self.advance();
            let right = self.parse_comparison_expression()?;
            left = Expression::BinaryOp {
                left: Box::new(left),
                op: BinaryOperator::And,
                right: Box::new(right),
            };
        }
        
        Ok(left)
    }
    
    fn parse_comparison_expression(&mut self) -> Result<Expression, MantisError> {
        let left = self.parse_additive_expression()?;
        
        let op = match self.current_token() {
            Token::Equal => BinaryOperator::Equal,
            Token::NotEqual => BinaryOperator::NotEqual,
            Token::Less => BinaryOperator::Less,
            Token::Greater => BinaryOperator::Greater,
            Token::LessEqual => BinaryOperator::LessEqual,
            Token::GreaterEqual => BinaryOperator::GreaterEqual,
            _ => return Ok(left),
        };
        
        self.advance();
        let right = self.parse_additive_expression()?;
        
        Ok(Expression::BinaryOp {
            left: Box::new(left),
            op,
            right: Box::new(right),
        })
    }
    
    fn parse_additive_expression(&mut self) -> Result<Expression, MantisError> {
        let mut left = self.parse_multiplicative_expression()?;
        
        loop {
            let op = match self.current_token() {
                Token::Plus => BinaryOperator::Add,
                Token::Minus => BinaryOperator::Subtract,
                _ => break,
            };
            
            self.advance();
            let right = self.parse_multiplicative_expression()?;
            left = Expression::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }
        
        Ok(left)
    }
    
    fn parse_multiplicative_expression(&mut self) -> Result<Expression, MantisError> {
        let mut left = self.parse_primary_expression()?;
        
        loop {
            let op = match self.current_token() {
                Token::Star => BinaryOperator::Multiply,
                Token::Slash => BinaryOperator::Divide,
                Token::Percent => BinaryOperator::Modulo,
                _ => break,
            };
            
            self.advance();
            let right = self.parse_primary_expression()?;
            left = Expression::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }
        
        Ok(left)
    }
    
    fn parse_primary_expression(&mut self) -> Result<Expression, MantisError> {
        match self.current_token().clone() {
            Token::IntegerLiteral(n) => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Integer(n as i32)))
            }
            Token::FloatLiteral(f) => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Double(f)))
            }
            Token::StringLiteral(s) => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Text(s)))
            }
            Token::True => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Boolean(true)))
            }
            Token::False => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Boolean(false)))
            }
            Token::Null => {
                self.advance();
                Ok(Expression::Literal(SqlValue::Null))
            }
            Token::Identifier(name) => {
                self.advance();
                
                // Check for function call
                if self.current_token() == &Token::LeftParen {
                    self.advance();
                    let args = if self.current_token() != &Token::RightParen {
                        self.parse_expression_list()?
                    } else {
                        Vec::new()
                    };
                    self.expect(Token::RightParen)?;
                    Ok(Expression::FunctionCall { name, args })
                } else {
                    Ok(Expression::Identifier(name))
                }
            }
            Token::LeftParen => {
                self.advance();
                
                // Check if this is a subquery (starts with SELECT)
                if self.current_token() == &Token::Select {
                    let subquery = self.parse_select()?;
                    self.expect(Token::RightParen)?;
                    
                    if let Statement::Select(select_stmt) = subquery {
                        return Ok(Expression::Subquery(Box::new(select_stmt)));
                    } else {
                        return Err(MantisError::ParseError(
                            "Expected SELECT statement in subquery".to_string()
                        ));
                    }
                }
                
                // Otherwise, it's a parenthesized expression
                let expr = self.parse_expression()?;
                self.expect(Token::RightParen)?;
                Ok(expr)
            }
            _ => Err(MantisError::ParseError(format!(
                "Unexpected token in expression: {:?}",
                self.current_token()
            ))),
        }
    }
    
    fn parse_expression_list(&mut self) -> Result<Vec<Expression>, MantisError> {
        let mut exprs = Vec::new();
        
        loop {
            exprs.push(self.parse_expression()?);
            
            if self.current_token() == &Token::Comma {
                self.advance();
            } else {
                break;
            }
        }
        
        Ok(exprs)
    }
    
    fn is_join_keyword(&self) -> bool {
        matches!(
            self.current_token(),
            Token::Join | Token::Inner | Token::Left | Token::Right | Token::Outer
        )
    }
    
    fn parse_join(&mut self) -> Result<Join, MantisError> {
        // Determine join type
        let join_type = match self.current_token() {
            Token::Inner => {
                self.advance();
                self.expect(Token::Join)?;
                JoinType::Inner
            }
            Token::Left => {
                self.advance();
                // Optional OUTER
                if self.current_token() == &Token::Outer {
                    self.advance();
                }
                self.expect(Token::Join)?;
                JoinType::Left
            }
            Token::Right => {
                self.advance();
                // Optional OUTER
                if self.current_token() == &Token::Outer {
                    self.advance();
                }
                self.expect(Token::Join)?;
                JoinType::Right
            }
            Token::Join => {
                self.advance();
                JoinType::Inner // Default to INNER JOIN
            }
            _ => {
                return Err(MantisError::ParseError(
                    "Expected JOIN keyword".to_string()
                ));
            }
        };
        
        // Parse table reference
        let table = self.parse_table_reference()?;
        
        // Parse ON condition
        self.expect(Token::On)?;
        let condition = self.parse_expression()?;
        
        Ok(Join {
            join_type,
            table,
            condition,
        })
    }
    
    fn parse_order_by_items(&mut self) -> Result<Vec<OrderByItem>, MantisError> {
        let mut items = Vec::new();
        
        loop {
            let expr = self.parse_expression()?;
            let ascending = true; // TODO: Parse ASC/DESC
            items.push(OrderByItem { expr, ascending });
            
            if self.current_token() == &Token::Comma {
                self.advance();
            } else {
                break;
            }
        }
        
        Ok(items)
    }
    
    fn parse_insert(&mut self) -> Result<Statement, MantisError> {
        // TODO: Implement INSERT parsing
        Err(MantisError::ParseError("INSERT not yet implemented".to_string()))
    }
    
    fn parse_update(&mut self) -> Result<Statement, MantisError> {
        // TODO: Implement UPDATE parsing
        Err(MantisError::ParseError("UPDATE not yet implemented".to_string()))
    }
    
    fn parse_delete(&mut self) -> Result<Statement, MantisError> {
        // TODO: Implement DELETE parsing
        Err(MantisError::ParseError("DELETE not yet implemented".to_string()))
    }
    
    fn parse_create(&mut self) -> Result<Statement, MantisError> {
        // TODO: Implement CREATE parsing
        Err(MantisError::ParseError("CREATE not yet implemented".to_string()))
    }
    
    fn parse_drop(&mut self) -> Result<Statement, MantisError> {
        // TODO: Implement DROP parsing
        Err(MantisError::ParseError("DROP not yet implemented".to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_simple_select() {
        let mut parser = Parser::new("SELECT * FROM users").unwrap();
        let stmt = parser.parse().unwrap();
        
        match stmt {
            Statement::Select(select) => {
                assert_eq!(select.columns.len(), 1);
                assert!(matches!(select.columns[0], SelectItem::Wildcard));
            }
            _ => panic!("Expected SELECT statement"),
        }
    }
    
    #[test]
    fn test_select_with_where() {
        let mut parser = Parser::new("SELECT id, name FROM users WHERE id = 1").unwrap();
        let stmt = parser.parse().unwrap();
        
        match stmt {
            Statement::Select(select) => {
                assert_eq!(select.columns.len(), 2);
                assert!(select.where_clause.is_some());
            }
            _ => panic!("Expected SELECT statement"),
        }
    }
}
