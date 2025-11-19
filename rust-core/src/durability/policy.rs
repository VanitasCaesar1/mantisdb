//! Policy
//!
//! Part of MantisDB - High-performance multi-model database.
//! See CONTRIBUTING.md for code standards and comment guidelines.

// Durability Policies
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DurabilityPolicy {
    None,
    Async,
    Sync,
    GroupCommit,
}

impl Default for DurabilityPolicy {
    fn default() -> Self {
        DurabilityPolicy::Sync
    }
}
