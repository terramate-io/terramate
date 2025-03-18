package script

import (
	"slices"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
)

type InfoEntry struct {
	ScriptCfg *hcl.Script
	Stacks    config.List[*config.SortableStack]
}

type Matcher struct {
	labels []string

	Results []*InfoEntry
}

func NewMatcher(labels []string) *Matcher {
	return &Matcher{
		labels:  labels,
		Results: nil,
	}
}

func (m *Matcher) Search(cfg *config.Root, stacks config.List[*config.SortableStack]) {
	rootScope := make(config.List[*config.SortableStack], len(stacks))
	copy(rootScope, stacks)

	m.searchRecursive(cfg.Tree(), &rootScope)
}

func tryGetScriptByName(cfg *config.Tree, labels []string) *hcl.Script {
	for _, s := range cfg.Node.Scripts {
		if slices.Equal(s.Labels, labels) {
			return s
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := maps.Keys(m)
	sort.Strings(ks)
	return ks
}

func (m *Matcher) searchRecursive(cfg *config.Tree, scope *config.List[*config.SortableStack]) {
	scriptCfg := tryGetScriptByName(cfg, m.labels)
	if scriptCfg != nil {
		m.addResult(cfg, scriptCfg, scope)
	} else {
		for _, k := range sortedKeys(cfg.Children) {
			m.searchRecursive(cfg.Children[k], scope)
		}
	}
}

func (m *Matcher) addResult(cfg *config.Tree, scriptCfg *hcl.Script, scope *config.List[*config.SortableStack]) {
	outerScope := config.List[*config.SortableStack]{}
	innerScope := config.List[*config.SortableStack]{}

	cfgdir := cfg.Dir()
	for _, st := range *scope {
		stackdir := st.Dir()

		if stackdir.HasDirPrefix(cfgdir.String()) {
			innerScope = append(innerScope, st)
		} else {
			outerScope = append(outerScope, st)
		}
	}

	// Modify the current scope
	*scope = outerScope

	newEntry := &InfoEntry{
		ScriptCfg: scriptCfg,
		// This may be updated at a deeper level as we move stacks to children
		Stacks: innerScope,
	}
	m.Results = append(m.Results, newEntry)

	for _, k := range sortedKeys(cfg.Children) {
		m.searchRecursive(cfg.Children[k], &newEntry.Stacks)
	}
}
