package cache

import (
	"sync"
)

// DependencyGraph tracks dependencies between cache entries
type DependencyGraph struct {
	// dependencies maps a key to its dependencies (what it depends on)
	dependencies map[string][]string
	// dependents maps a key to its dependents (what depends on it)
	dependents map[string][]string
	mutex      sync.RWMutex
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
	}
}

// AddDependency adds a dependency relationship
// key depends on dependency
func (dg *DependencyGraph) AddDependency(key, dependency string) {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	// Add to dependencies map
	if deps, exists := dg.dependencies[key]; exists {
		// Check if dependency already exists
		for _, dep := range deps {
			if dep == dependency {
				return // Already exists
			}
		}
		dg.dependencies[key] = append(deps, dependency)
	} else {
		dg.dependencies[key] = []string{dependency}
	}

	// Add to dependents map
	if deps, exists := dg.dependents[dependency]; exists {
		// Check if dependent already exists
		for _, dep := range deps {
			if dep == key {
				return // Already exists
			}
		}
		dg.dependents[dependency] = append(deps, key)
	} else {
		dg.dependents[dependency] = []string{key}
	}
}

// RemoveDependency removes a specific dependency relationship
func (dg *DependencyGraph) RemoveDependency(key, dependency string) {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	// Remove from dependencies map
	if deps, exists := dg.dependencies[key]; exists {
		newDeps := make([]string, 0, len(deps))
		for _, dep := range deps {
			if dep != dependency {
				newDeps = append(newDeps, dep)
			}
		}
		if len(newDeps) == 0 {
			delete(dg.dependencies, key)
		} else {
			dg.dependencies[key] = newDeps
		}
	}

	// Remove from dependents map
	if deps, exists := dg.dependents[dependency]; exists {
		newDeps := make([]string, 0, len(deps))
		for _, dep := range deps {
			if dep != key {
				newDeps = append(newDeps, dep)
			}
		}
		if len(newDeps) == 0 {
			delete(dg.dependents, dependency)
		} else {
			dg.dependents[dependency] = newDeps
		}
	}
}

// RemoveNode removes a node and all its relationships
func (dg *DependencyGraph) RemoveNode(key string) {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	// Remove all dependencies of this key
	if deps, exists := dg.dependencies[key]; exists {
		for _, dep := range deps {
			dg.removeDependentUnsafe(dep, key)
		}
		delete(dg.dependencies, key)
	}

	// Remove all dependents of this key
	if deps, exists := dg.dependents[key]; exists {
		for _, dep := range deps {
			dg.removeDependencyUnsafe(dep, key)
		}
		delete(dg.dependents, key)
	}
}

// GetDependencies returns all dependencies of a key
func (dg *DependencyGraph) GetDependencies(key string) []string {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	if deps, exists := dg.dependencies[key]; exists {
		// Return a copy to avoid race conditions
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// GetDependents returns all dependents of a key
func (dg *DependencyGraph) GetDependents(key string) []string {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	if deps, exists := dg.dependents[key]; exists {
		// Return a copy to avoid race conditions
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// GetAllDependents returns all transitive dependents of a key
func (dg *DependencyGraph) GetAllDependents(key string) []string {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	visited := make(map[string]bool)
	var result []string

	dg.getAllDependentsRecursive(key, visited, &result)

	return result
}

// HasCycle checks if adding a dependency would create a cycle
func (dg *DependencyGraph) HasCycle(key, dependency string) bool {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	// Check if dependency depends on key (directly or indirectly)
	visited := make(map[string]bool)
	return dg.hasCycleRecursive(dependency, key, visited)
}

// GetStats returns statistics about the dependency graph
func (dg *DependencyGraph) GetStats() DependencyStats {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	totalNodes := len(dg.dependencies)
	if len(dg.dependents) > totalNodes {
		totalNodes = len(dg.dependents)
	}

	var totalEdges int
	for _, deps := range dg.dependencies {
		totalEdges += len(deps)
	}

	return DependencyStats{
		TotalNodes: totalNodes,
		TotalEdges: totalEdges,
	}
}

// DependencyStats holds statistics about the dependency graph
type DependencyStats struct {
	TotalNodes int
	TotalEdges int
}

// Private helper methods

func (dg *DependencyGraph) removeDependentUnsafe(dependency, key string) {
	if deps, exists := dg.dependents[dependency]; exists {
		newDeps := make([]string, 0, len(deps))
		for _, dep := range deps {
			if dep != key {
				newDeps = append(newDeps, dep)
			}
		}
		if len(newDeps) == 0 {
			delete(dg.dependents, dependency)
		} else {
			dg.dependents[dependency] = newDeps
		}
	}
}

func (dg *DependencyGraph) removeDependencyUnsafe(key, dependency string) {
	if deps, exists := dg.dependencies[key]; exists {
		newDeps := make([]string, 0, len(deps))
		for _, dep := range deps {
			if dep != dependency {
				newDeps = append(newDeps, dep)
			}
		}
		if len(newDeps) == 0 {
			delete(dg.dependencies, key)
		} else {
			dg.dependencies[key] = newDeps
		}
	}
}

func (dg *DependencyGraph) getAllDependentsRecursive(key string, visited map[string]bool, result *[]string) {
	if visited[key] {
		return
	}
	visited[key] = true

	if deps, exists := dg.dependents[key]; exists {
		for _, dep := range deps {
			*result = append(*result, dep)
			dg.getAllDependentsRecursive(dep, visited, result)
		}
	}
}

func (dg *DependencyGraph) hasCycleRecursive(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}

	if visited[current] {
		return false
	}
	visited[current] = true

	if deps, exists := dg.dependencies[current]; exists {
		for _, dep := range deps {
			if dg.hasCycleRecursive(dep, target, visited) {
				return true
			}
		}
	}

	return false
}
