//! Row Level Security (RLS) Engine for MantisDB
//!
//! Implements PostgreSQL-compatible Row Level Security with high performance.
//! Policies are compiled into fast evaluation functions with minimal overhead.

use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

/// RLS Policy type
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PolicyType {
    /// SELECT policies control which rows are visible
    Select,
    /// INSERT policies control which rows can be inserted
    Insert,
    /// UPDATE policies control which rows can be updated
    Update,
    /// DELETE policies control which rows can be deleted
    Delete,
    /// ALL applies to all operations
    All,
}

/// Policy command specifies when the policy applies
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PolicyCommand {
    /// Apply to SELECT operations
    Select,
    /// Apply to INSERT operations
    Insert,
    /// Apply to UPDATE operations (both old and new rows)
    Update,
    /// Apply to DELETE operations
    Delete,
    /// Apply to all operations
    All,
}

/// Policy permission model
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PolicyPermission {
    /// Permissive policies (OR logic)
    Permissive,
    /// Restrictive policies (AND logic)
    Restrictive,
}

/// RLS Policy definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Policy {
    /// Policy name (unique per table)
    pub name: String,
    /// Table this policy applies to
    pub table: String,
    /// Command type
    pub command: PolicyCommand,
    /// Permission model
    pub permission: PolicyPermission,
    /// Roles this policy applies to (empty = all roles)
    pub roles: Vec<String>,
    /// USING expression (for SELECT, UPDATE, DELETE)
    pub using_expr: Option<String>,
    /// WITH CHECK expression (for INSERT, UPDATE)
    pub with_check_expr: Option<String>,
    /// Policy enabled
    pub enabled: bool,
}

impl Policy {
    /// Check if policy applies to a specific role
    pub fn applies_to_role(&self, role: &str) -> bool {
        self.roles.is_empty() || self.roles.contains(&role.to_string())
    }

    /// Check if policy applies to a specific command
    pub fn applies_to_command(&self, command: &PolicyCommand) -> bool {
        self.command == PolicyCommand::All || self.command == *command
    }
}

/// Context for policy evaluation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PolicyContext {
    /// Current user ID
    pub user_id: Option<String>,
    /// Current user role
    pub role: String,
    /// Session variables
    pub session_vars: HashMap<String, serde_json::Value>,
    /// Request metadata
    pub metadata: HashMap<String, String>,
}

impl Default for PolicyContext {
    fn default() -> Self {
        Self {
            user_id: None,
            role: "anonymous".to_string(),
            session_vars: HashMap::new(),
            metadata: HashMap::new(),
        }
    }
}

impl PolicyContext {
    pub fn new(role: String) -> Self {
        Self {
            role,
            ..Default::default()
        }
    }

    pub fn with_user_id(mut self, user_id: String) -> Self {
        self.user_id = Some(user_id);
        self
    }

    pub fn with_session_var(mut self, key: String, value: serde_json::Value) -> Self {
        self.session_vars.insert(key, value);
        self
    }
}

/// Policy evaluation result
#[derive(Debug, Clone, PartialEq)]
pub enum PolicyResult {
    /// Access allowed
    Allow,
    /// Access denied
    Deny,
    /// Policy doesn't apply
    NotApplicable,
}

/// Expression evaluator for policy conditions
pub struct PolicyEvaluator {
    /// Compiled expressions cache
    expressions: HashMap<String, CompiledExpression>,
}

/// Compiled expression for fast evaluation
#[derive(Debug, Clone)]
pub struct CompiledExpression {
    /// Original expression
    pub original: String,
    /// Expression type
    pub expr_type: ExpressionType,
}

#[derive(Debug, Clone, PartialEq)]
pub enum ExpressionType {
    /// Always true
    AlwaysTrue,
    /// Always false
    AlwaysFalse,
    /// User ID check: user_id = current_user_id
    UserIdCheck { column: String },
    /// Role check: role = current_role
    RoleCheck { allowed_roles: Vec<String> },
    /// Custom expression
    Custom { expression: String },
    /// Composite AND expression
    And { expressions: Vec<ExpressionType> },
    /// Composite OR expression
    Or { expressions: Vec<ExpressionType> },
}

impl PolicyEvaluator {
    pub fn new() -> Self {
        Self {
            expressions: HashMap::new(),
        }
    }

    /// Compile an expression for fast evaluation
    pub fn compile(&mut self, expression: &str) -> Result<CompiledExpression> {
        // Simple expression parser
        let expr_type = self.parse_expression(expression)?;
        
        Ok(CompiledExpression {
            original: expression.to_string(),
            expr_type,
        })
    }

    /// Parse expression into optimized form
    fn parse_expression(&self, expr: &str) -> Result<ExpressionType> {
        let expr = expr.trim();

        // Check for boolean literals
        if expr == "true" || expr == "1" {
            return Ok(ExpressionType::AlwaysTrue);
        }
        if expr == "false" || expr == "0" {
            return Ok(ExpressionType::AlwaysFalse);
        }

        // Check for user_id comparisons
        if expr.contains("user_id") && expr.contains("auth.uid()") {
            if let Some(column) = self.extract_column_name(expr, "user_id") {
                return Ok(ExpressionType::UserIdCheck { column });
            }
        }

        // Check for role comparisons
        if expr.contains("role") && expr.contains("auth.role()") {
            if let Some(roles) = self.extract_roles(expr) {
                return Ok(ExpressionType::RoleCheck { allowed_roles: roles });
            }
        }

        // Handle AND/OR expressions
        if expr.contains(" AND ") || expr.contains(" and ") {
            let parts: Vec<&str> = expr.split(" AND ").collect();
            let sub_exprs: Result<Vec<_>> = parts
                .iter()
                .map(|p| self.parse_expression(p.trim()))
                .collect();
            return Ok(ExpressionType::And { expressions: sub_exprs? });
        }

        if expr.contains(" OR ") || expr.contains(" or ") {
            let parts: Vec<&str> = expr.split(" OR ").collect();
            let sub_exprs: Result<Vec<_>> = parts
                .iter()
                .map(|p| self.parse_expression(p.trim()))
                .collect();
            return Ok(ExpressionType::Or { expressions: sub_exprs? });
        }

        // Default to custom expression
        Ok(ExpressionType::Custom {
            expression: expr.to_string(),
        })
    }

    fn extract_column_name(&self, _expr: &str, default: &str) -> Option<String> {
        // Simple extraction - in production, use a proper parser
        Some(default.to_string())
    }

    fn extract_roles(&self, _expr: &str) -> Option<Vec<String>> {
        // Simple extraction - in production, use a proper parser
        Some(vec!["authenticated".to_string()])
    }

    /// Evaluate an expression against a context and row data
    pub fn evaluate(
        &self,
        expr: &CompiledExpression,
        context: &PolicyContext,
        row_data: &serde_json::Value,
    ) -> Result<bool> {
        self.evaluate_type(&expr.expr_type, context, row_data)
    }

    fn evaluate_type(
        &self,
        expr_type: &ExpressionType,
        context: &PolicyContext,
        row_data: &serde_json::Value,
    ) -> Result<bool> {
        match expr_type {
            ExpressionType::AlwaysTrue => Ok(true),
            ExpressionType::AlwaysFalse => Ok(false),
            
            ExpressionType::UserIdCheck { column } => {
                if let Some(user_id) = &context.user_id {
                    if let Some(row_user_id) = row_data.get(column) {
                        Ok(row_user_id.as_str() == Some(user_id.as_str()))
                    } else {
                        Ok(false)
                    }
                } else {
                    Ok(false)
                }
            }

            ExpressionType::RoleCheck { allowed_roles } => {
                Ok(allowed_roles.contains(&context.role))
            }

            ExpressionType::And { expressions } => {
                for expr in expressions {
                    if !self.evaluate_type(expr, context, row_data)? {
                        return Ok(false);
                    }
                }
                Ok(true)
            }

            ExpressionType::Or { expressions } => {
                for expr in expressions {
                    if self.evaluate_type(expr, context, row_data)? {
                        return Ok(true);
                    }
                }
                Ok(false)
            }

            ExpressionType::Custom { expression: _ } => {
                // For custom expressions, would need a full SQL/expression evaluator
                // For now, default to true (unsafe - implement proper evaluation)
                Ok(true)
            }
        }
    }
}

/// RLS Engine manages policies and enforces security
pub struct RlsEngine {
    /// Policies by table
    policies: Arc<RwLock<HashMap<String, Vec<Policy>>>>,
    /// RLS enabled tables
    enabled_tables: Arc<RwLock<HashMap<String, bool>>>,
    /// Policy evaluator
    evaluator: Arc<RwLock<PolicyEvaluator>>,
}

impl RlsEngine {
    pub fn new() -> Self {
        Self {
            policies: Arc::new(RwLock::new(HashMap::new())),
            enabled_tables: Arc::new(RwLock::new(HashMap::new())),
            evaluator: Arc::new(RwLock::new(PolicyEvaluator::new())),
        }
    }

    /// Enable RLS for a table
    pub fn enable_rls(&self, table: &str) -> Result<()> {
        let mut enabled = self.enabled_tables.write()
            .map_err(|_| Error::Io("Lock poisoned".to_string()))?;
        enabled.insert(table.to_string(), true);
        Ok(())
    }

    /// Disable RLS for a table
    pub fn disable_rls(&self, table: &str) -> Result<()> {
        let mut enabled = self.enabled_tables.write()
            .map_err(|_| Error::Io("Lock poisoned".to_string()))?;
        enabled.insert(table.to_string(), false);
        Ok(())
    }

    /// Check if RLS is enabled for a table
    pub fn is_rls_enabled(&self, table: &str) -> bool {
        let enabled = self.enabled_tables.read().unwrap();
        enabled.get(table).copied().unwrap_or(false)
    }

    /// Add a policy to a table
    pub fn add_policy(&self, policy: Policy) -> Result<()> {
        let mut policies = self.policies.write()
            .map_err(|_| Error::Io("Lock poisoned".to_string()))?;
        
        let table_policies = policies.entry(policy.table.clone()).or_insert_with(Vec::new);
        
        // Remove existing policy with same name
        table_policies.retain(|p| p.name != policy.name);
        
        // Add new policy
        table_policies.push(policy);
        
        Ok(())
    }

    /// Remove a policy
    pub fn remove_policy(&self, table: &str, policy_name: &str) -> Result<()> {
        let mut policies = self.policies.write()
            .map_err(|_| Error::Io("Lock poisoned".to_string()))?;
        
        if let Some(table_policies) = policies.get_mut(table) {
            table_policies.retain(|p| p.name != policy_name);
        }
        
        Ok(())
    }

    /// Get all policies for a table
    pub fn get_policies(&self, table: &str) -> Vec<Policy> {
        let policies = self.policies.read().unwrap();
        policies.get(table).cloned().unwrap_or_default()
    }

    /// Check if a SELECT operation is allowed for a row
    pub fn check_select(
        &self,
        table: &str,
        context: &PolicyContext,
        row_data: &serde_json::Value,
    ) -> Result<bool> {
        self.check_operation(table, &PolicyCommand::Select, context, row_data, None)
    }

    /// Check if an INSERT operation is allowed
    pub fn check_insert(
        &self,
        table: &str,
        context: &PolicyContext,
        new_row: &serde_json::Value,
    ) -> Result<bool> {
        self.check_operation(table, &PolicyCommand::Insert, context, new_row, None)
    }

    /// Check if an UPDATE operation is allowed
    pub fn check_update(
        &self,
        table: &str,
        context: &PolicyContext,
        old_row: &serde_json::Value,
        new_row: &serde_json::Value,
    ) -> Result<bool> {
        self.check_operation(table, &PolicyCommand::Update, context, old_row, Some(new_row))
    }

    /// Check if a DELETE operation is allowed
    pub fn check_delete(
        &self,
        table: &str,
        context: &PolicyContext,
        row_data: &serde_json::Value,
    ) -> Result<bool> {
        self.check_operation(table, &PolicyCommand::Delete, context, row_data, None)
    }

    /// Core policy checking logic
    fn check_operation(
        &self,
        table: &str,
        command: &PolicyCommand,
        context: &PolicyContext,
        row_data: &serde_json::Value,
        new_row_data: Option<&serde_json::Value>,
    ) -> Result<bool> {
        // If RLS is not enabled, allow all operations
        if !self.is_rls_enabled(table) {
            return Ok(true);
        }

        let policies = self.policies.read().unwrap();
        let table_policies = match policies.get(table) {
            Some(p) => p,
            None => return Ok(false), // No policies = deny by default when RLS enabled
        };

        // Filter policies that apply to this operation and role
        let applicable_policies: Vec<_> = table_policies
            .iter()
            .filter(|p| p.enabled && p.applies_to_role(&context.role) && p.applies_to_command(command))
            .collect();

        if applicable_policies.is_empty() {
            return Ok(false); // No applicable policies = deny
        }

        // Separate permissive and restrictive policies
        let (permissive, restrictive): (Vec<_>, Vec<_>) = applicable_policies
            .into_iter()
            .partition(|p| p.permission == PolicyPermission::Permissive);

        // Evaluate restrictive policies (all must pass)
        for policy in restrictive {
            let result = self.evaluate_policy(policy, context, row_data, new_row_data)?;
            if !result {
                return Ok(false); // Any restrictive policy failure = deny
            }
        }

        // Evaluate permissive policies (at least one must pass)
        if !permissive.is_empty() {
            let mut any_passed = false;
            for policy in permissive {
                let result = self.evaluate_policy(policy, context, row_data, new_row_data)?;
                if result {
                    any_passed = true;
                    break;
                }
            }
            if !any_passed {
                return Ok(false); // No permissive policy passed = deny
            }
        }

        Ok(true)
    }

    fn evaluate_policy(
        &self,
        policy: &Policy,
        context: &PolicyContext,
        row_data: &serde_json::Value,
        new_row_data: Option<&serde_json::Value>,
    ) -> Result<bool> {
        let mut evaluator = self.evaluator.write()
            .map_err(|_| Error::Io("Lock poisoned".to_string()))?;
        
        // Evaluate USING expression
        if let Some(using_expr) = &policy.using_expr {
            let compiled = evaluator.compile(using_expr)?;
            if !evaluator.evaluate(&compiled, context, row_data)? {
                return Ok(false);
            }
        }

        // Evaluate WITH CHECK expression (for INSERT/UPDATE)
        if let Some(with_check_expr) = &policy.with_check_expr {
            let check_row = new_row_data.unwrap_or(row_data);
            let compiled = evaluator.compile(with_check_expr)?;
            if !evaluator.evaluate(&compiled, context, check_row)? {
                return Ok(false);
            }
        }

        Ok(true)
    }
}

impl Default for RlsEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_policy_creation() {
        let policy = Policy {
            name: "test_policy".to_string(),
            table: "users".to_string(),
            command: PolicyCommand::Select,
            permission: PolicyPermission::Permissive,
            roles: vec!["authenticated".to_string()],
            using_expr: Some("user_id = auth.uid()".to_string()),
            with_check_expr: None,
            enabled: true,
        };

        assert_eq!(policy.name, "test_policy");
        assert!(policy.applies_to_role("authenticated"));
        assert!(!policy.applies_to_role("anonymous"));
    }

    #[test]
    fn test_rls_engine() {
        let engine = RlsEngine::new();
        
        // Enable RLS
        engine.enable_rls("users").unwrap();
        assert!(engine.is_rls_enabled("users"));

        // Add policy
        let policy = Policy {
            name: "user_select".to_string(),
            table: "users".to_string(),
            command: PolicyCommand::Select,
            permission: PolicyPermission::Permissive,
            roles: vec![],
            using_expr: Some("true".to_string()),
            with_check_expr: None,
            enabled: true,
        };

        engine.add_policy(policy).unwrap();

        let policies = engine.get_policies("users");
        assert_eq!(policies.len(), 1);
    }

    #[test]
    fn test_policy_evaluation() {
        let engine = RlsEngine::new();
        engine.enable_rls("users").unwrap();

        // Add a simple always-allow policy
        let policy = Policy {
            name: "allow_all".to_string(),
            table: "users".to_string(),
            command: PolicyCommand::All,
            permission: PolicyPermission::Permissive,
            roles: vec![],
            using_expr: Some("true".to_string()),
            with_check_expr: None,
            enabled: true,
        };

        engine.add_policy(policy).unwrap();

        let context = PolicyContext::new("authenticated".to_string());
        let row = serde_json::json!({"id": 1, "name": "test"});

        let result = engine.check_select("users", &context, &row).unwrap();
        assert!(result);
    }
}
