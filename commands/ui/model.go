// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/scaffold/manifest"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// ViewState represents which view is currently active.
type ViewState int

// ViewCloudLogin and the following constants enumerate the possible view states.
const (
	ViewCloudLogin     ViewState = iota // Cloud login prompt (shown first)
	ViewEnvSelect                       // Initial environment selection
	ViewOverview                        // Main overview
	ViewCreateSelect                    // Collection/bundle selection (pre-inputs)
	ViewCreateInput                     // Create-bundle wizard flow (inputs page)
	ViewReconfigSelect                  // Selecting an existing bundle to reconfigure
	ViewReconfigInput                   // Editing the selected existing bundle's inputs
	ViewEdit                            // Edit pending change
	ViewPromoteSelect                   // Selecting a bundle to promote to the current environment
	ViewPromoteInput                    // Selecting a bundle to promote to the current environment
)

// FocusArea represents which section has focus in the overview.
type FocusArea int

// FocusCommands and the following constants enumerate the overview focus areas.
const (
	FocusCommands FocusArea = iota
	FocusSummary
)

// BundleSelectPage represents the current page in the bundle selection view.
type BundleSelectPage int

// BundleSelectCollection and the following constants enumerate the bundle selection pages.
const (
	BundleSelectCollection BundleSelectPage = iota
	BundleSelectBundle
)

// ObjectEditFrame captures the form state when suspending for nested object editing.
type ObjectEditFrame struct {
	inputsForm    InputsForm
	objectInputID string // The ID of the object input being edited
	objectName    string // Human-readable name of the object input (for breadcrumb)
}

// CreateFrame captures the wizard state when suspending for nested bundle creation.
type CreateFrame struct {
	bundleSelectPage  BundleSelectPage
	selectedCollIdx   int
	selectedBundleIdx int
	inputsForm        InputsForm
	parentBundleName  string // name of the bundle being configured (for UI context)
}

// InputOption represents a selectable option for select/multiselect inputs.
type InputOption struct {
	Label string
	Value cty.Value
}

// EngineState contains common engine state used throughout all layers.
//
// None of these attributes should change after initialization,
// with the exception of Registry, which will contain the pending bundles we created,
// so we can already reference them.
type EngineState struct {
	Context         context.Context
	CLI             commands.CLI
	WorkingDir      string
	Root            *config.Root
	Evalctx         *eval.Context // This is the base evalctx. Should be cloned instead of modifie directly.
	ResolveAPI      resolve.API
	Registry        *Registry
	LocalBundleDefs []config.BundleDefinitionEntry
	Collections     []*manifest.Collection
	CLIConfig       cliconfig.Config
	AgentAddress    string
}

// Model is the main BubbleTea model for the prompt UI.
type Model struct {
	// Layout
	width  int
	height int

	// Common state
	EngineState *EngineState

	// View state
	viewState ViewState

	// Cloud login state

	cloudLoginButtonIdx int
	cloudLoginLoading   bool
	cloudSignupMsg      string

	// Environment selection state
	envCursor   int
	selectedEnv *config.Environment

	// Overview state
	focus      FocusArea
	commandIdx int
	commands   []string

	summaryCursor         int           // Selected row in the pending changes table
	summaryButtonIdx      int           // 0 = Save, 1 = Discard (inline buttons on Pending Changes title)
	summaryOnButtons      bool          // true when focus is on the inline buttons (not the list)
	changesApplied        bool          // true after Save — shows success message instead of list
	savedChanges          []SavedChange // snapshot of changes at the time Save was pressed
	changeLog             []string      // cumulative log of all saved changes across the session
	confirmingDiscard     bool          // true when showing the discard confirmation prompt
	discardConfirmIdx     int           // 0 = Yes, 1 = No
	confirmingExit        bool          // true when showing exit confirmation (unsaved changes)
	exitConfirmIdx        int           // 0 = Yes, 1 = No
	confirmingCreateExit  bool          // true when showing wizard exit confirmation
	createExitConfirmIdx  int           // 0 = Yes, 1 = No
	confirmingEditDiscard bool          // true when showing edit-change cancel confirmation
	editDiscardConfirmIdx int           // 0 = Yes, 1 = No
	editingChangeIdx      int           // Index of the change being edited (for ViewEditCreate)

	// Bundle selection state
	bundleSelectPage       BundleSelectPage
	selectedCollIdx        int
	selectedBundleIdx      int
	selectedBundleDefEntry *config.BundleDefinitionEntry
	selectedBundleSource   string
	inputsForm             InputsForm
	lastUsedCollIdx        int // -1 means no last used
	hasLastUsedColl        bool

	// Bundle reference / nested creation state
	createStack    []CreateFrame // Stack of suspended wizard states
	nestedRefClass string        // When non-empty, we're creating a bundle for this class

	// Object input nested editing state
	objectEditStack []ObjectEditFrame // Stack for nested object input editing

	// Reconfigure state
	reconfigBundles []*config.Bundle // Filtered bundles (excludes pending changes), rebuilt on entry
	reconfigCursor  int              // Cursor in reconfigBundles
	reconfigBundle  *config.Bundle   // The bundle currently being reconfigured

	// Promote state
	promoteBundles []*config.Bundle // Bundles eligible for promotion, rebuilt on entry
	promoteCursor  int              // Cursor in promoteBundles
	promoteBundle  *config.Bundle   // The bundle currently being promoted

	// Transient status
	currentErr   error // Shown on the help line, cleared on next keypress
	saveErr      error // Shown below pending changes, cleared on next keypress
	ctrlCPending bool  // true after first ctrl+c press, reset after 1s

	// Result
	err       error
	cancelled bool
}

const uiWidth = 100
const uiContentHeight = 26                          // Max visible content lines inside bordered panels
const uiInputsPanelHeight = uiContentHeight + 2 + 4 // Match select-panel outer height (content + padding + border)

// NewModel creates a new prompt model.
func NewModel(est *EngineState) Model {
	vp := viewport.New(uiWidth-6-2, uiContentHeight)
	vp.KeyMap = viewport.KeyMap{}

	var initialViewState ViewState

	if _, err := os.Stat(cliauth.CredentialFile(est.CLIConfig)); err == nil {
		if len(est.Registry.Environments) > 0 {
			initialViewState = ViewEnvSelect
		} else {
			initialViewState = ViewOverview
		}
	} else {
		initialViewState = ViewCloudLogin
	}

	return Model{
		EngineState: est,

		viewState: initialViewState,
		commands: []string{
			"Create",
			"Reconfigure",
			"Promote",
			"Quit",
		},
		focus: FocusCommands,
	}
}

// inputsToValueMap converts a map[string]cty.Value to map[string]cty.Value,
// unwrapping the {"value": v} object that EvalInputs wraps each input in.
func inputsToValueMap(inputs map[string]cty.Value) map[string]cty.Value {
	out := make(map[string]cty.Value, len(inputs))
	for k, v := range inputs {
		out[k] = v.GetAttr("value")
	}
	return out
}

// ctrlCResetMsg is sent after the double-press window expires.
type ctrlCResetMsg struct{}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case ctrlCResetMsg:
		m.ctrlCPending = false
		return m, nil

	case cloudLoginResultMsg:
		m.cloudLoginLoading = false
		if msg.err != nil {
			m.currentErr = msg.err
			return m, nil
		}
		m.viewState = m.nextViewAfterCloudLogin()
		return m, textarea.Blink

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			if m.ctrlCPending {
				m.cancelled = true
				return m, tea.Quit
			}
			m.ctrlCPending = true
			return m, tea.Tick(time.Second, func(time.Time) tea.Msg {
				return ctrlCResetMsg{}
			})
		}

		switch m.viewState {
		case ViewCloudLogin:
			return m.updateCloudLogin(msg)
		case ViewEnvSelect:
			return m.updateEnvSelect(msg)
		case ViewCreateSelect:
			return m.updateCreateSelect(msg)
		case ViewCreateInput:
			return m.updateCreateInput(msg)
		case ViewEdit:
			return m.updateEdit(msg)
		case ViewReconfigSelect:
			return m.updateReconfigSelect(msg)
		case ViewReconfigInput:
			return m.updateReconfigInput(msg)
		case ViewPromoteSelect:
			return m.updatePromoteSelect(msg)
		case ViewPromoteInput:
			return m.updatePromoteInput(msg)
		default:
			return m.updateOverview(msg)
		}

	default:
		switch m.viewState {
		case ViewCreateInput, ViewReconfigInput, ViewEdit:
			var cmd tea.Cmd
			m.inputsForm, cmd = m.inputsForm.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// View renders the current view.
func (m Model) View() string {
	switch m.viewState {
	case ViewCloudLogin:
		return m.renderCloudLoginView()
	case ViewEnvSelect:
		return m.renderEnvSelectView()
	case ViewCreateSelect:
		return m.renderBundleSelectView()
	case ViewCreateInput:
		return m.renderCreateInputView()
	case ViewEdit:
		return m.renderEditChangeView()
	case ViewReconfigSelect:
		return m.renderReconfigSelectView()
	case ViewReconfigInput:
		return m.renderReconfigInputView()
	case ViewPromoteSelect:
		return m.renderPromoteSelectView()
	case ViewPromoteInput:
		return m.renderPromoteInputView()
	default:
		return m.renderOverviewView()
	}
}

// PendingChanges returns the list of changes that have been saved but not yet applied.
func (m *Model) PendingChanges() []Change {
	return m.EngineState.Registry.PendingChanges
}

// SetPendingChanges replaces the current list of pending changes.
func (m *Model) SetPendingChanges(c []Change) {
	m.EngineState.Registry.PendingChanges = c
}

// ProposedChanges returns the list of proposed (unsaved) changes in the current session.
func (m *Model) ProposedChanges() []Change {
	return m.EngineState.Registry.ProposedChanges
}

// SetProposedChanges replaces the current list of proposed changes.
func (m *Model) SetProposedChanges(c []Change) {
	m.EngineState.Registry.ProposedChanges = c
}

// keyMap defines the key bindings for the prompt UI.
type keyMap struct {
	Quit   key.Binding
	Tab    key.Binding
	Enter  key.Binding
	Escape key.Binding
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch section"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
}

func (m Model) updateError(err error) (tea.Model, tea.Cmd) {
	m.currentErr = err
	return m, nil
}

func (m Model) finalHelpText(base string) string {
	if m.ctrlCPending {
		return "press ctrl+c again to exit"
	}
	return base
}

func displayNameFromAlias(alias, name string) string {
	if strings.HasSuffix(alias, ":"+name) {
		return name
	}
	return alias
}

// Registry wraps config.Registry with session-local state for bundles pending to be created
// during the current session.
type Registry struct {
	*config.Registry

	// The registry owns these list as a common place to store all available bundles.
	PendingChanges  []Change
	ProposedChanges []Change
}

// MatchingBundleOptions returns a merged list of existing and session-created
// bundles that match the given class ID and environment.
func (r *Registry) MatchingBundleOptions(classID string, env *config.Environment) []BundleOption {
	var options []BundleOption
	for _, b := range r.Bundles {
		if classID != b.DefinitionMetadata.Class {
			continue
		}
		if b.Environment != nil && env != nil {
			if env.ID != b.Environment.ID {
				continue
			}
		}
		opt := BundleOption{
			Name:  b.Name,
			Alias: b.Alias,
		}
		if b.Environment != nil {
			opt.EnvID = b.Environment.ID
		}
		options = append(options, opt)
	}
	for _, b := range append(r.PendingChanges, r.ProposedChanges...) {
		if classID != b.BundleDefEntry.Metadata.Class {
			continue
		}
		if b.Env != nil && env != nil {
			if env.ID != b.Env.ID {
				continue
			}
		}
		opt := BundleOption{
			Name:  b.Name,
			Alias: b.Alias,
		}
		if b.Env != nil {
			opt.EnvID = b.Env.ID
		}
		options = append(options, opt)
	}
	return options
}

// IsBundleUnique checks that no existing or pending bundle conflicts with the given alias and class.
func (r *Registry) IsBundleUnique(alias, classID, hostPath string) error {
	if alias != "" {
		for _, b := range r.Bundles {
			bundleHostPath := b.Info.HostPath()
			if classID == b.DefinitionMetadata.Class && alias == b.Alias {
				return errors.E("A bundle with alias %q already exists at %s", b.Alias, bundleHostPath)
			}
		}
		for _, b := range append(r.PendingChanges, r.ProposedChanges...) {
			if b.MarkedForReplacement {
				continue
			}
			if classID == b.BundleDefEntry.Metadata.Class && alias == b.Alias {
				return errors.E("A bundle with alias %q is already pending to be created at %s", b.Alias, b.HostPath)
			}
		}
	}

	if hostPath != "" {
		_, err := os.Stat(hostPath)
		if err == nil {
			return errors.E("A file already exists at the target output path %s", hostPath)
		}
	}

	return nil
}
