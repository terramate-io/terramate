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

package dag

import (
	"fmt"
	"sort"

	"github.com/madlambda/spells/errutil"
)

type (
	// ID of nodes
	ID string

	// DAG is a Directed-Acyclic Graph
	DAG struct {
		dag    map[ID][]ID
		values map[ID]interface{}
		cycles map[ID]bool

		validated bool
	}
)

const (
	ErrDuplicateNode errutil.Error = "duplicate node"
	ErrNodeNotFound  errutil.Error = "node not found"
	ErrCycleDetected errutil.Error = "cycle detected"
)

// New creates a new empty Directed-Acyclic-Graph.
func New() *DAG {
	return &DAG{
		dag:    make(map[ID][]ID),
		values: make(map[ID]interface{}),
	}
}

// AddNode adds a new node to the dag. The lists of before and after
// defines its edge nodes.
func (d *DAG) AddNode(id ID, value interface{}, before, after []ID) error {
	if _, ok := d.values[id]; ok {
		return errutil.Chain(
			ErrDuplicateNode,
			fmt.Errorf("adding node id %q", id),
		)
	}

	for _, bid := range before {
		if _, ok := d.dag[bid]; !ok {
			d.dag[bid] = []ID{}
		}

		d.addEdge(bid, id)
	}

	if _, ok := d.dag[id]; !ok {
		d.dag[id] = []ID{}
	}

	d.addEdges(id, after)
	d.values[id] = value
	d.validated = false
	return nil
}

func (d *DAG) addEdges(from ID, toids []ID) {
	for _, to := range toids {
		d.addEdge(from, to)
	}
}

func (d *DAG) addEdge(from, to ID) {
	fromEdges, ok := d.dag[from]
	if !ok {
		panic("internal error: empty list of edges must exist at this point")
	}

	if !IDList(fromEdges).contains(to) {
		fromEdges = append(fromEdges, to)
	}

	d.dag[from] = fromEdges
}

// Validate the DAG looking for cycles.
func (d *DAG) Validate() (reason string, err error) {
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

func (d *DAG) validateNode(id ID, children []ID) (string, error) {
	found, reason := d.hasCycle([]ID{id}, children, fmt.Sprintf("%s ->", id))
	if found {
		d.cycles[id] = true
		return reason, errutil.Chain(
			ErrCycleDetected,
			fmt.Errorf("checking node id %q", id),
		)
	}

	return "", nil
}

func (d *DAG) hasCycle(branch []ID, children []ID, reason string) (bool, string) {
	for _, id := range branch {
		if IDList(children).contains(id) {
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
func (d *DAG) IDs() []ID {
	idlist := make(IDList, 0, len(d.dag))
	for id := range d.dag {
		idlist = append(idlist, id)
	}
	sort.Sort(idlist)
	return idlist
}

// Node returns the node with the given id.
func (d *DAG) Node(id ID) (interface{}, error) {
	v, ok := d.values[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return v, nil
}

// ChildrenOf returns the list of node ids that are children of the given id.
func (d *DAG) ChildrenOf(id ID) []ID {
	return d.dag[id]
}

func (d *DAG) HasCycle(id ID) bool {
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
func (d *DAG) Order() []ID {
	order := []ID{}
	visited := map[ID]struct{}{}
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

func (d *DAG) walkFrom(id ID, do func(id ID)) {
	children := d.dag[id]
	for _, tid := range sortedIds(children) {
		d.walkFrom(tid, do)
	}

	do(id)
}

func sortedIds(ids []ID) IDList {
	idlist := make(IDList, 0, len(ids))
	for _, id := range ids {
		idlist = append(idlist, id)
	}

	sort.Sort(idlist)
	return idlist
}

type IDList []ID

func (ids IDList) contains(other ID) bool {
	for _, id := range ids {
		if id == other {
			return true
		}
	}

	return false
}

func (ids IDList) Len() int           { return len(ids) }
func (ids IDList) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }
func (ids IDList) Less(i, j int) bool { return ids[i] < ids[j] }
