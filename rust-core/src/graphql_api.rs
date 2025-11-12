//! GraphQL API Layer
//!
//! GraphQL interface for MantisDB with automatic schema generation

use crate::error::{Error, Result};
use serde::{Serialize, Deserialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

/// GraphQL API server
pub struct GraphQLApi {
    inner: Arc<RwLock<GraphQLInner>>,
}

struct GraphQLInner {
    schemas: HashMap<String, GraphQLSchema>,
    resolvers: HashMap<String, Box<dyn Resolver>>,
}

/// GraphQL schema definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphQLSchema {
    pub types: Vec<TypeDefinition>,
    pub queries: Vec<QueryDefinition>,
    pub mutations: Vec<MutationDefinition>,
    pub subscriptions: Vec<SubscriptionDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TypeDefinition {
    pub name: String,
    pub fields: Vec<FieldDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldDefinition {
    pub name: String,
    pub field_type: String,
    pub nullable: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryDefinition {
    pub name: String,
    pub return_type: String,
    pub arguments: Vec<ArgumentDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MutationDefinition {
    pub name: String,
    pub return_type: String,
    pub arguments: Vec<ArgumentDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SubscriptionDefinition {
    pub name: String,
    pub return_type: String,
    pub arguments: Vec<ArgumentDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArgumentDefinition {
    pub name: String,
    pub arg_type: String,
    pub required: bool,
}

trait Resolver: Send + Sync {
    fn resolve(&self, args: HashMap<String, serde_json::Value>) -> Result<serde_json::Value>;
}

impl GraphQLApi {
    /// Create a new GraphQL API
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(GraphQLInner {
                schemas: HashMap::new(),
                resolvers: HashMap::new(),
            })),
        }
    }
    
    /// Generate schema from SQL tables
    pub fn generate_schema_from_tables(&self, tables: Vec<TableSchema>) -> Result<GraphQLSchema> {
        let mut types = Vec::new();
        let mut queries = Vec::new();
        let mut mutations = Vec::new();
        
        for table in tables {
            // Create GraphQL type from table
            let type_def = TypeDefinition {
                name: Self::to_pascal_case(&table.name),
                fields: table.columns.iter().map(|col| {
                    FieldDefinition {
                        name: col.name.clone(),
                        field_type: Self::sql_to_graphql_type(&col.data_type),
                        nullable: col.nullable,
                    }
                }).collect(),
            };
            types.push(type_def);
            
            // Generate queries
            queries.push(QueryDefinition {
                name: format!("get{}", Self::to_pascal_case(&table.name)),
                return_type: Self::to_pascal_case(&table.name),
                arguments: vec![ArgumentDefinition {
                    name: "id".to_string(),
                    arg_type: "ID".to_string(),
                    required: true,
                }],
            });
            
            queries.push(QueryDefinition {
                name: format!("list{}", Self::pluralize(&Self::to_pascal_case(&table.name))),
                return_type: format!("[{}]", Self::to_pascal_case(&table.name)),
                arguments: vec![
                    ArgumentDefinition {
                        name: "limit".to_string(),
                        arg_type: "Int".to_string(),
                        required: false,
                    },
                    ArgumentDefinition {
                        name: "offset".to_string(),
                        arg_type: "Int".to_string(),
                        required: false,
                    },
                ],
            });
            
            // Generate mutations
            mutations.push(MutationDefinition {
                name: format!("create{}", Self::to_pascal_case(&table.name)),
                return_type: Self::to_pascal_case(&table.name),
                arguments: vec![ArgumentDefinition {
                    name: "input".to_string(),
                    arg_type: format!("Create{}Input", Self::to_pascal_case(&table.name)),
                    required: true,
                }],
            });
            
            mutations.push(MutationDefinition {
                name: format!("update{}", Self::to_pascal_case(&table.name)),
                return_type: Self::to_pascal_case(&table.name),
                arguments: vec![
                    ArgumentDefinition {
                        name: "id".to_string(),
                        arg_type: "ID".to_string(),
                        required: true,
                    },
                    ArgumentDefinition {
                        name: "input".to_string(),
                        arg_type: format!("Update{}Input", Self::to_pascal_case(&table.name)),
                        required: true,
                    },
                ],
            });
            
            mutations.push(MutationDefinition {
                name: format!("delete{}", Self::to_pascal_case(&table.name)),
                return_type: "Boolean".to_string(),
                arguments: vec![ArgumentDefinition {
                    name: "id".to_string(),
                    arg_type: "ID".to_string(),
                    required: true,
                }],
            });
        }
        
        Ok(GraphQLSchema {
            types,
            queries,
            mutations,
            subscriptions: vec![],
        })
    }
    
    /// Execute a GraphQL query
    pub fn execute_query(&self, query: &str) -> Result<serde_json::Value> {
        // Parse and execute GraphQL query
        // This is a simplified stub - real implementation would use a GraphQL parser
        
        Ok(serde_json::json!({
            "data": {
                "message": "GraphQL query executed successfully"
            }
        }))
    }
    
    /// Generate SDL (Schema Definition Language)
    pub fn generate_sdl(&self, schema: &GraphQLSchema) -> String {
        let mut sdl = String::new();
        
        // Types
        for type_def in &schema.types {
            sdl.push_str(&format!("type {} {{\n", type_def.name));
            for field in &type_def.fields {
                let nullable = if field.nullable { "" } else { "!" };
                sdl.push_str(&format!("  {}: {}{}\n", field.name, field.field_type, nullable));
            }
            sdl.push_str("}\n\n");
        }
        
        // Query type
        if !schema.queries.is_empty() {
            sdl.push_str("type Query {\n");
            for query in &schema.queries {
                let args = query.arguments.iter()
                    .map(|arg| {
                        let required = if arg.required { "!" } else { "" };
                        format!("{}: {}{}", arg.name, arg.arg_type, required)
                    })
                    .collect::<Vec<_>>()
                    .join(", ");
                
                sdl.push_str(&format!("  {}({}): {}\n", query.name, args, query.return_type));
            }
            sdl.push_str("}\n\n");
        }
        
        // Mutation type
        if !schema.mutations.is_empty() {
            sdl.push_str("type Mutation {\n");
            for mutation in &schema.mutations {
                let args = mutation.arguments.iter()
                    .map(|arg| {
                        let required = if arg.required { "!" } else { "" };
                        format!("{}: {}{}", arg.name, arg.arg_type, required)
                    })
                    .collect::<Vec<_>>()
                    .join(", ");
                
                sdl.push_str(&format!("  {}({}): {}\n", mutation.name, args, mutation.return_type));
            }
            sdl.push_str("}\n\n");
        }
        
        sdl
    }
    
    // Helper methods
    
    fn sql_to_graphql_type(sql_type: &str) -> String {
        match sql_type.to_uppercase().as_str() {
            "INT" | "INTEGER" | "BIGINT" => "Int".to_string(),
            "REAL" | "FLOAT" | "DOUBLE" => "Float".to_string(),
            "TEXT" | "VARCHAR" | "CHAR" => "String".to_string(),
            "BOOLEAN" | "BOOL" => "Boolean".to_string(),
            _ => "String".to_string(),
        }
    }
    
    fn to_pascal_case(s: &str) -> String {
        s.split('_')
            .map(|word| {
                let mut chars = word.chars();
                match chars.next() {
                    None => String::new(),
                    Some(first) => first.to_uppercase().collect::<String>() + chars.as_str(),
                }
            })
            .collect()
    }
    
    fn pluralize(s: &str) -> String {
        if s.ends_with('y') && s.len() > 1 {
            format!("{}ies", &s[..s.len()-1])
        } else if s.ends_with('s') {
            format!("{}es", s)
        } else {
            format!("{}s", s)
        }
    }
}

/// Table schema for SQL to GraphQL conversion
#[derive(Debug, Clone)]
pub struct TableSchema {
    pub name: String,
    pub columns: Vec<ColumnSchema>,
}

#[derive(Debug, Clone)]
pub struct ColumnSchema {
    pub name: String,
    pub data_type: String,
    pub nullable: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_sql_to_graphql_type() {
        assert_eq!(GraphQLApi::sql_to_graphql_type("INT"), "Int");
        assert_eq!(GraphQLApi::sql_to_graphql_type("TEXT"), "String");
        assert_eq!(GraphQLApi::sql_to_graphql_type("BOOLEAN"), "Boolean");
    }
    
    #[test]
    fn test_to_pascal_case() {
        assert_eq!(GraphQLApi::to_pascal_case("user_profile"), "UserProfile");
        assert_eq!(GraphQLApi::to_pascal_case("order"), "Order");
    }
    
    #[test]
    fn test_pluralize() {
        assert_eq!(GraphQLApi::pluralize("User"), "Users");
        assert_eq!(GraphQLApi::pluralize("Category"), "Categories");
    }
    
    #[test]
    fn test_schema_generation() {
        let api = GraphQLApi::new();
        
        let tables = vec![
            TableSchema {
                name: "users".to_string(),
                columns: vec![
                    ColumnSchema {
                        name: "id".to_string(),
                        data_type: "INT".to_string(),
                        nullable: false,
                    },
                    ColumnSchema {
                        name: "name".to_string(),
                        data_type: "TEXT".to_string(),
                        nullable: false,
                    },
                ],
            },
        ];
        
        let schema = api.generate_schema_from_tables(tables).unwrap();
        assert!(!schema.types.is_empty());
        assert!(!schema.queries.is_empty());
        assert!(!schema.mutations.is_empty());
    }
}
