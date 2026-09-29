package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"

	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/scope"
)

var (
	nodeKeyPattern = regexp.MustCompile("^[a-z][a-z0-9_]*$")
	validRoles     = map[NodeRole]bool{
		RoleSupervisor: true,
		RoleStructure:  true,
		RoleWriter:     true,
		RolePolisher:   true,
		RoleReviewer:   true,
		RoleTool:       true,
	}
)

// ValidateDAG validates the workflow manifest graph according to the R1 contract:
// 1..64 nodes, <= 128 edges, Kahn topological sort (no cycles, no self loops),
// valid roles, valid bindings to ancestor nodes, and role/version consistency.
func ValidateDAG(manifest WorkflowManifest) error {
	nodes := manifest.Nodes
	if len(nodes) < 1 || len(nodes) > 64 {
		return ErrInvalidDAG(map[string]any{"reason": "node count must be between 1 and 64"})
	}

	nodeMap := make(map[string]ManifestNode, len(nodes))
	nodeIndices := make(map[string]int, len(nodes))
	totalEdges := 0

	for i, node := range nodes {
		if len(node.NodeKey) < 1 || len(node.NodeKey) > 64 || !nodeKeyPattern.MatchString(node.NodeKey) {
			return ErrInvalidDAG(map[string]any{"reason": "invalid node_key", "node": node.NodeKey})
		}
		if _, exists := nodeMap[node.NodeKey]; exists {
			return ErrInvalidDAG(map[string]any{"reason": "duplicate node_key", "node": node.NodeKey})
		}
		role := NodeRole(node.Role)
		if !validRoles[role] {
			return ErrInvalidDAG(map[string]any{"reason": "invalid role", "node": node.NodeKey, "role": node.Role})
		}

		if role == RoleTool {
			if node.ToolVersionID == nil || !scope.ValidID(*node.ToolVersionID) {
				return ErrInvalidDAG(map[string]any{"reason": "tool role requires valid tool_version_id", "node": node.NodeKey})
			}
			if node.DefinitionVersionID != nil {
				return ErrInvalidDAG(map[string]any{"reason": "tool role forbids definition_version_id", "node": node.NodeKey})
			}
		} else {
			if node.ToolVersionID != nil {
				return ErrInvalidDAG(map[string]any{"reason": "non-tool role forbids tool_version_id", "node": node.NodeKey})
			}
			if node.DefinitionVersionID != nil && !scope.ValidID(*node.DefinitionVersionID) {
				return ErrInvalidDAG(map[string]any{"reason": "invalid definition_version_id", "node": node.NodeKey})
			}
		}

		nodeMap[node.NodeKey] = node
		nodeIndices[node.NodeKey] = i
		totalEdges += len(node.DependsOn)
	}

	if totalEdges > 128 {
		return ErrInvalidDAG(map[string]any{"reason": "edge count exceeds 128", "count": totalEdges})
	}

	// Validate depends_on references and uniqueness
	inDegree := make(map[string]int, len(nodes))
	adjacency := make(map[string][]string, len(nodes)) // dependency -> dependents

	for _, node := range nodes {
		inDegree[node.NodeKey] = 0
	}

	for _, node := range nodes {
		seenDep := make(map[string]bool, len(node.DependsOn))
		for _, dep := range node.DependsOn {
			if dep == node.NodeKey {
				return ErrInvalidDAG(map[string]any{"reason": "self dependency forbidden", "node": node.NodeKey})
			}
			if _, exists := nodeMap[dep]; !exists {
				return ErrInvalidDAG(map[string]any{"reason": "dangling dependency reference", "node": node.NodeKey, "dependency": dep})
			}
			if seenDep[dep] {
				return ErrInvalidDAG(map[string]any{"reason": "duplicate dependency reference", "node": node.NodeKey, "dependency": dep})
			}
			seenDep[dep] = true

			inDegree[node.NodeKey]++
			adjacency[dep] = append(adjacency[dep], node.NodeKey)
		}
	}

	// Kahn topological sort
	queue := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if inDegree[node.NodeKey] == 0 {
			queue = append(queue, node.NodeKey)
		}
	}

	visitedCount := 0
	topologicalOrder := make([]string, 0, len(nodes))

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++
		topologicalOrder = append(topologicalOrder, curr)

		for _, dependent := range adjacency[curr] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if visitedCount != len(nodes) {
		return ErrInvalidDAG(map[string]any{"reason": "cycle detected in workflow DAG"})
	}

	// Transitive closure of ancestors for input binding validation
	ancestors := make(map[string]map[string]bool, len(nodes))
	for _, node := range nodes {
		ancestors[node.NodeKey] = make(map[string]bool)
	}

	// Process in topological order so dependencies are populated before dependents
	for _, nodeKey := range topologicalOrder {
		node := nodeMap[nodeKey]
		for _, dep := range node.DependsOn {
			ancestors[nodeKey][dep] = true
			for anc := range ancestors[dep] {
				ancestors[nodeKey][anc] = true
			}
		}
	}

	// Validate input bindings
	for _, node := range nodes {
		if len(node.InputBindings) > 64 {
			return ErrInvalidDAG(map[string]any{"reason": "input bindings exceed 64", "node": node.NodeKey})
		}
		seenParam := make(map[string]bool, len(node.InputBindings))
		for _, binding := range node.InputBindings {
			if len(binding.TargetParameter) < 1 || len(binding.TargetParameter) > 120 {
				return ErrInvalidDAG(map[string]any{"reason": "invalid target_parameter", "node": node.NodeKey})
			}
			if seenParam[binding.TargetParameter] {
				return ErrInvalidDAG(map[string]any{"reason": "duplicate target_parameter binding", "node": node.NodeKey, "parameter": binding.TargetParameter})
			}
			seenParam[binding.TargetParameter] = true

			if !ancestors[node.NodeKey][binding.SourceNode] {
				return ErrInvalidDAG(map[string]any{"reason": "binding source_node must be an ancestor in DAG", "node": node.NodeKey, "source": binding.SourceNode})
			}
			if len(binding.SourcePointer) > 256 {
				return ErrInvalidDAG(map[string]any{"reason": "source_pointer too long", "node": node.NodeKey})
			}
			if binding.SourcePointer != "" && binding.SourcePointer[0] != '/' {
				return ErrInvalidDAG(map[string]any{"reason": "source_pointer must be empty or start with /", "node": node.NodeKey})
			}
		}
	}

	return nil
}

// ComputeManifestHash returns the deterministic SHA-256 hex digest of the canonical manifest JSON.
func ComputeManifestHash(manifest WorkflowManifest) (string, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	canonical, err := idempotency.CanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
