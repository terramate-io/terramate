// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dag provides the Directed-Acyclic-Graph (DAG) primitives required by
// Terramate.
// The nodes can be added by providing both the descendants and ancestors of
// each node but only the descendant -> ancestors relationship is kept.
package dag

import (
	"fmt"
	"sort"

	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

type (
	// ID of nodes
	ID string

	// DAG is a Directed-Acyclic Graph
	DAG struct {
		// dag is a map of descendantID -> []ancestorID
		dag    map[ID][]ID
		values map[ID]interface{}
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
func New() *DAG {
	return &DAG{
		dag:    make(map[ID][]ID),
		values: make(map[ID]interface{}),
	}
}

// AddNode adds a new node to the dag. The lists of descendants and ancestors
// defines its edge.
// The value is anything related to the node that needs to be retrieved later
// when processing the DAG.
func (d *DAG) AddNode(id ID, value interface{}, descendants, ancestors []ID) error {
	logger := log.With().
		Str("action", "AddNode()").
		Logger()

	if _, ok := d.values[id]; ok {
		return errors.E(ErrDuplicateNode,
			fmt.Sprintf("adding node id %q", id),
		)
	}

	for _, bid := range descendants {
		if _, ok := d.dag[bid]; !ok {
			d.dag[bid] = []ID{}
		}

		logger.Trace().
			Str("from", string(bid)).
			Str("to", string(id)).
			Msg("Add edge.")
		d.addAncestor(bid, id)
	}

	if _, ok := d.dag[id]; !ok {
		d.dag[id] = []ID{}
	}

	logger.Trace().
		Str("id", string(id)).
		Msg("Add edges.")
	d.addAncestors(id, ancestors)
	d.values[id] = value
	d.validated = false
	return nil
}

func (d *DAG) addAncestors(node ID, ancestorIDs []ID) {
	for _, to := range ancestorIDs {
		log.Trace().
			Str("action", "addAncestors()").
			Str("node", string(node)).
			Str("ancestor", string(to)).
			Msg("Add edges.")
		d.addAncestor(node, to)
	}
}

func (d *DAG) addAncestor(node, ancestor ID) {
	nodeAncestors, ok := d.dag[node]
	if !ok {
		panic("internal error: empty list of edges must exist at this point")
	}

	if !idList(nodeAncestors).contains(ancestor) {
		log.Trace().
			Str("action", "addAncestor()").
			Str("node", string(node)).
			Str("ancestor", string(ancestor)).
			Msg("Append edge.")
		nodeAncestors = append(nodeAncestors, ancestor)
	}

	d.dag[node] = nodeAncestors
}

// Validate the DAG looking for cycles.
func (d *DAG) Validate() (reason string, err error) {
	d.cycles = make(map[ID]bool)
	d.validated = true

	for _, id := range d.IDs() {
		log.Trace().
			Str("action", "Validate()").
			Str("id", string(id)).
			Msg("Validate node.")
		reason, err := d.validateNode(id, d.dag[id])
		if err != nil {
			return reason, err
		}
	}
	return "", nil
}

func (d *DAG) validateNode(id ID, children []ID) (string, error) {
	log.Trace().
		Str("action", "validateNode()").
		Str("id", string(id)).
		Msg("Check if has cycle.")
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

func (d *DAG) hasCycle(branch []ID, children []ID, reason string) (bool, string) {
	for _, id := range branch {
		log.Trace().
			Str("action", "hasCycle()").
			Str("id", string(id)).
			Msg("Check if id is present in children.")
		if idList(children).contains(id) {
			d.cycles[id] = true
			return true, fmt.Sprintf("%s %s", reason, id)
		}
	}

	for _, tid := range sortedIds(children) {
		tlist := d.dag[tid]
		log.Trace().
			Str("action", "hasCycle()").
			Str("id", string(tid)).
			Msg("Check if id has cycle.")
		found, reason := d.hasCycle(append(branch, tid), tlist, fmt.Sprintf("%s %s ->", reason, tid))
		if found {
			return true, reason
		}
	}

	return false, ""
}

// IDs returns the sorted list of node ids.
func (d *DAG) IDs() []ID {
	idlist := make(idList, 0, len(d.dag))
	for id := range d.dag {
		idlist = append(idlist, id)
	}

	log.Trace().
		Str("action", "IDs()").
		Msg("Sort node ids.")
	sort.Sort(idlist)
	return idlist
}

// Node returns the node with the given id.
func (d *DAG) Node(id ID) (interface{}, error) {
	v, ok := d.values[id]
	if !ok {
		return nil, errors.E(ErrNodeNotFound)
	}
	return v, nil
}

// AncestorsOf returns the list of ancestor node ids of the given id.
func (d *DAG) AncestorsOf(id ID) []ID {
	return d.dag[id]
}

// HasCycle returns true if the DAG has a cycle.
func (d *DAG) HasCycle(id ID) bool {
	if !d.validated {
		log.Trace().
			Str("action", "HasCycle()").
			Str("id", string(id)).
			Msg("Validate.")
		_, err := d.Validate()
		if err == nil {
			return false
		}
	}

	return d.cycles[id]
}

// Order returns the topological order of the DAG. The node ids are
// lexicographic sorted whenever possible to give a consistent output.
func (d *DAG) Order() []ID {
	order := []ID{}
	visited := Visited{}
	for _, id := range d.IDs() {
		if _, ok := visited[id]; ok {
			continue
		}
		log.Trace().
			Str("action", "Order()").
			Str("id", string(id)).
			Msg("Walk from current id.")
		d.walkFrom(id, func(id ID) {
			if _, ok := visited[id]; !ok {
				log.Trace().
					Str("action", "Order()").
					Str("id", string(id)).
					Msg("Append to ordered array.")
				order = append(order, id)
			}

			visited[id] = struct{}{}
		})

		visited[id] = struct{}{}
	}
	return order
}

func (d *DAG) walkFrom(id ID, do func(id ID)) {
	children := d.dag[id]
	for _, tid := range sortedIds(children) {
		log.Trace().
			Str("action", "walkFrom()").
			Str("id", string(id)).
			Msg("Walk from current id.")
		d.walkFrom(tid, do)
	}

	do(id)
}

func sortedIds(ids []ID) idList {
	idlist := make(idList, 0, len(ids))
	for _, id := range ids {
		idlist = append(idlist, id)
	}

	log.Trace().
		Str("action", "sortedIds()").
		Msg("Sort ids.")
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
