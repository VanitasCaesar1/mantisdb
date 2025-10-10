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
