package graph

import (
	"container/list"

	"github.com/Qxy0happy/codegraph-go/internal/types"
)

// TraversalStep is one hop in a call path.
type TraversalStep struct {
	Node types.Node `json:"node"`
	Edge types.Edge `json:"edge"`
}

// TraversalResult is a chain of steps from source to target.
type TraversalResult struct {
	Steps []TraversalStep `json:"steps"`
}

// GetCallersBFS returns callers of nodeID up to maxDepth, using BFS.
func (d *DB) GetCallersBFS(nodeID string, maxDepth int) ([]TraversalStep, error) {
	return d.traverseBFS(nodeID, maxDepth, true /* incoming */)
}

// GetCalleesBFS returns callees of nodeID up to maxDepth, using BFS.
func (d *DB) GetCalleesBFS(nodeID string, maxDepth int) ([]TraversalStep, error) {
	return d.traverseBFS(nodeID, maxDepth, false /* outgoing */)
}

// FindPath attempts to find a path from srcID to dstID using BFS.
func (d *DB) FindPath(srcID, dstID string, edgeKinds []types.EdgeKind) (*TraversalResult, error) {
	type bfsNode struct {
		id    string
		steps []TraversalStep
	}

	visited := make(map[string]bool)
	queue := list.New()
	queue.PushBack(&bfsNode{id: srcID, steps: nil})
	visited[srcID] = true

	for queue.Len() > 0 {
		front := queue.Remove(queue.Front()).(*bfsNode)
		if front.id == dstID {
			return &TraversalResult{Steps: front.steps}, nil
		}

		edges, err := d.GetEdgesBySource(front.id, edgeKinds...)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if visited[e.Target] {
				continue
			}
			visited[e.Target] = true
			targetNode, err := d.GetNodeByID(e.Target)
			if err != nil {
				continue
			}
			newSteps := append([]TraversalStep{}, front.steps...)
			newSteps = append(newSteps, TraversalStep{Node: *targetNode, Edge: e})
			queue.PushBack(&bfsNode{id: e.Target, steps: newSteps})
		}
	}
	return nil, nil // no path found
}

// Impact returns all nodes reachable from nodeID via any edge kind, up to maxDepth.
func (d *DB) Impact(nodeID string, maxDepth int) ([]TraversalStep, error) {
	return d.traverseBFS(nodeID, maxDepth, false /* outgoing */)
}

// traverseBFS implements breadth-first search in the edge graph.
// When incoming=true, follows edges pointing TO nodeID (callers).
// When incoming=false, follows edges FROM nodeID (callees, impact).
func (d *DB) traverseBFS(nodeID string, maxDepth int, incoming bool) ([]TraversalStep, error) {
	type queueItem struct {
		id    string
		depth int
		edge  types.Edge
	}

	if maxDepth <= 0 {
		maxDepth = 10
	}

	visited := make(map[string]int) // nodeID → depth (min)
	queue := list.New()
	queue.PushBack(&queueItem{id: nodeID, depth: 0})
	visited[nodeID] = 0

	var result []TraversalStep

	for queue.Len() > 0 {
		front := queue.Remove(queue.Front()).(*queueItem)
		if front.depth >= maxDepth {
			continue
		}

		var edges []types.Edge
		var err error
		if incoming {
			edges, err = d.GetEdgesByTarget(front.id)
		} else {
			edges, err = d.GetEdgesBySource(front.id)
		}
		if err != nil {
			return nil, err
		}

		for _, e := range edges {
			nextID := e.Target
			if incoming {
				nextID = e.Source
			}

			if prevDepth, seen := visited[nextID]; seen && prevDepth <= front.depth+1 {
				continue
			}
			visited[nextID] = front.depth + 1

			node, err := d.GetNodeByID(nextID)
			if err != nil {
				continue
			}
			result = append(result, TraversalStep{Node: *node, Edge: e})

			queue.PushBack(&queueItem{id: nextID, depth: front.depth + 1, edge: e})
		}
	}

	return result, nil
}
