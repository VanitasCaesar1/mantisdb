// Deadlock Detection
use super::types::*;
use std::collections::{HashMap, HashSet};

pub struct DeadlockDetector;

impl DeadlockDetector {
    pub fn detect_cycle(
        wait_for_graph: &HashMap<TransactionId, Vec<TransactionId>>,
        start: TransactionId,
    ) -> Option<Vec<TransactionId>> {
        let mut visited = HashSet::new();
        let mut rec_stack = Vec::new();
        
        if Self::dfs(start, wait_for_graph, &mut visited, &mut rec_stack) {
            Some(rec_stack)
        } else {
            None
        }
    }
    
    fn dfs(
        node: TransactionId,
        graph: &HashMap<TransactionId, Vec<TransactionId>>,
        visited: &mut HashSet<TransactionId>,
        rec_stack: &mut Vec<TransactionId>,
    ) -> bool {
        visited.insert(node);
        rec_stack.push(node);
        
        if let Some(neighbors) = graph.get(&node) {
            for &neighbor in neighbors {
                if !visited.contains(&neighbor) {
                    if Self::dfs(neighbor, graph, visited, rec_stack) {
                        return true;
                    }
                } else if rec_stack.contains(&neighbor) {
                    return true;
                }
            }
        }
        
        rec_stack.pop();
        false
    }
}
