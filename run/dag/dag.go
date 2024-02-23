// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package dag provides the Directed-Acyclic-Graph (DAG) primitives required by
// Terramate.
// The nodes can be added by providing both the descendants and ancestors of
// each node but only the descendant -> ancestors relationship is kept.
package dag

import (
	"fmt"
	"sort"

	"github.com/terramate-io/terramate/errors"
)

type (
	// ID of nodes
	ID string

	// DAG is a Directed-Acyclic Graph
	DAG[V any] struct {
		// dag is a map of descendantID -> []ancestorID
		dag    map[ID][]ID
		values map[ID]V
		cycles map[ID]bool

		validated bool
	}

	// Visited in a map of visited dag nodes by id.
	// Note: it's not concurrent-safe.
	Visited map[ID]struct{}
)

// Errors returned by operations on the DAG.
const (
	ErrDuplicateNode errors.Kind = "duplicate node"
	ErrNodeNotFound  errors.Kind = "node not found"
	ErrCycleDetected errors.Kind = "cycle detected"
)

// New creates a new empty Directed-Acyclic-Graph.
func New[V any]() *DAG[V] {
	return &DAG[V]{
		dag:    make(map[ID][]ID),
		values: make(map[ID]V),
	}
}

// AddNode adds a new node to the dag. The lists of descendants and ancestors
// defines its edge.
// The value is anything related to the node that needs to be retrieved later
// when processing the DAG.
func (d *DAG[V]) AddNode(id ID, value V, descendants, ancestors []ID) error {
	if _, ok := d.values[id]; ok {
		return errors.E(ErrDuplicateNode,
			fmt.Sprintf("adding node id %q", id),
		)
	}

	for _, bid := range descendants {
		if _, ok := d.dag[bid]; !ok {
			d.dag[bid] = []ID{}
		}

		d.addAncestor(bid, id)
	}

	if _, ok := d.dag[id]; !ok {
		d.dag[id] = []ID{}
	}

	d.addAncestors(id, ancestors)
	d.values[id] = value
	d.validated = false
	return nil
}

func (d *DAG[V]) addAncestors(node ID, ancestorIDs []ID) {
	for _, ancestor := range ancestorIDs {
		d.addAncestor(node, ancestor)
	}
}

func (d *DAG[V]) addAncestor(node, ancestor ID) {
	nodeAncestors, ok := d.dag[node]
	if !ok {
		panic("internal error: empty list of edges must exist at this point")
	}

	if !idList(nodeAncestors).contains(ancestor) {
		nodeAncestors = append(nodeAncestors, ancestor)
	}

	d.dag[node] = nodeAncestors
}

// Validate the DAG looking for cycles.
func (d *DAG[V]) Validate() (reason string, err error) {
	d.cycles = make(map[ID]bool)
	d.validated = true

	for _, id := range d.IDs() {
		reason, err := d.validateNode(id, d.dag[id])
		if err != nil {
			return reason, err
		}
	}
	return "", nil
}

// Reduce removes nodes that match the given predicate.
// When a node is removed, all edges to it are replaced by edges to its children,
// so the order within the graph is preserved.
func (d *DAG[V]) Reduce(predicate func(id ID) bool) {
	visited := Visited{}

	shouldRemove := make(map[ID]bool, len(d.dag))
	ids := d.IDs()

	// Cache predicates
	for _, id := range ids {
		shouldRemove[id] = predicate(id)
	}

	// Remove nodes from as children and replace with grandchildren
	for _, id := range ids {
		d.walkFrom(id, func(id ID) {
			if _, found := visited[id]; found {
				return
			}
			visited[id] = struct{}{}

			newChildren := []ID{}
			for _, cid := range d.dag[id] {
				if shouldRemove[cid] {
					grandchildren := d.dag[cid]
					newChildren = append(newChildren, grandchildren...)
				} else {
					newChildren = append(newChildren, cid)
				}
			}

			d.dag[id] = newChildren
		})
	}

	// Remove nodes themselves
	for id, remove := range shouldRemove {
		if remove {
			delete(d.dag, id)
			delete(d.values, id)
		}
	}
}

func (d *DAG[V]) validateNode(id ID, children []ID) (string, error) {
	found, reason := d.hasCycle([]ID{id}, children, fmt.Sprintf("%s ->", id))
	if found {
		d.cycles[id] = true
		return reason, errors.E(
			ErrCycleDetected,
			fmt.Sprintf("checking node id %q", id),
		)
	}

	return "", nil
}

func (d *DAG[V]) hasCycle(branch []ID, children []ID, reason string) (bool, string) {
	for _, id := range branch {
		if idList(children).contains(id) {
			d.cycles[id] = true
			return true, fmt.Sprintf("%s %s", reason, id)
		}
	}

	for _, tid := range sortedIds(children) {
		tlist := d.dag[tid]
		found, reason := d.hasCycle(append(branch, tid), tlist, fmt.Sprintf("%s %s ->", reason, tid))
		if found {
			return true, reason
		}
	}

	return false, ""
}

// IDs returns the sorted list of node ids.
func (d *DAG[V]) IDs() []ID {
	idlist := make(idList, 0, len(d.dag))
	for id := range d.dag {
		idlist = append(idlist, id)
	}

	sort.Sort(idlist)
	return idlist
}

// Node returns the node with the given id.
func (d *DAG[V]) Node(id ID) (V, error) {
	v, ok := d.values[id]
	if !ok {
		var v V
		return v, errors.E(ErrNodeNotFound)
	}
	return v, nil
}

// AncestorsOf returns the list of ancestor node ids of the given id.
func (d *DAG[V]) AncestorsOf(id ID) []ID {
	return d.dag[id]
}

// HasCycle returns true if the DAG has a cycle.
func (d *DAG[V]) HasCycle(id ID) bool {
	if !d.validated {
		_, err := d.Validate()
		if err == nil {
			return false
		}
	}

	return d.cycles[id]
}

// Order returns the topological order of the DAG. The node ids are
// lexicographic sorted whenever possible to give a consistent output.
func (d *DAG[V]) Order() []ID {
	order := []ID{}
	visited := Visited{}
	for _, id := range d.IDs() {
		if _, ok := visited[id]; ok {
			continue
		}
		d.walkFrom(id, func(id ID) {
			if _, ok := visited[id]; !ok {
				order = append(order, id)
			}

			visited[id] = struct{}{}
		})

		visited[id] = struct{}{}
	}
	return order
}

func (d *DAG[V]) walkFrom(id ID, do func(id ID)) {
	children := d.dag[id]
	for _, tid := range sortedIds(children) {
		d.walkFrom(tid, do)
	}

	do(id)
}

func sortedIds(ids []ID) idList {
	idlist := make(idList, 0, len(ids))
	for _, id := range ids {
		idlist = append(idlist, id)
	}

	sort.Sort(idlist)
	return idlist
}

type idList []ID

func (ids idList) contains(other ID) bool {
	for _, id := range ids {
		if id == other {
			return true
		}
	}

	return false
}

func (ids idList) Len() int           { return len(ids) }
func (ids idList) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }
func (ids idList) Less(i, j int) bool { return ids[i] < ids[j] }

// Transform transforms a DAG of D's to a DAG of S's by applying the given function to each node.
// Afterwards, source DAG must be discarded.
func Transform[D, S any](from *DAG[S], f func(id ID, v S) (D, error)) (*DAG[D], error) {
	to := &DAG[D]{
		dag:       from.dag,
		values:    make(map[ID]D, len(from.values)),
		cycles:    from.cycles,
		validated: from.validated,
	}

	for id, v := range from.values {
		if fv, err := f(id, v); err == nil {
			to.values[id] = fv
		} else {
			return nil, err
		}
	}

	// Discard from
	from.dag = nil
	from.values = nil
	from.cycles = nil
	from.validated = false

	return to, nil
}
