// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate"
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
	ViewCloudLogin      ViewState = iota // Cloud login prompt (shown first)
	ViewEnvSelect                        // Unused, kept to preserve iota values
	ViewOverview                         // Main overview
	ViewCreateSelect                     // Flat bundle selection (pre-inputs)
	ViewCreateEnvSelect                  // Environment selection after bundle pick (Create only)
	ViewCreateInput                      // Create-bundle wizard flow (inputs page)
	ViewReconfigSelect                   // Selecting an existing bundle to reconfigure
	ViewReconfigInput                    // Editing the selected existing bundle's inputs
	ViewPromoteSelect                    // Selecting a bundle to promote
	ViewPromoteInput                     // Editing promoted bundle's inputs
)

// FocusArea represents which section has focus in the overview.
type FocusArea int

// FocusCommands and the following constants enumerate the overview focus areas.
const (
	FocusCommands FocusArea = iota
	FocusSummary
)

// ObjectEditFrame captures the form state when suspending for nested object editing.
type ObjectEditFrame struct {
	inputsForm    InputsForm
	objectInputID string // The ID of the object input being edited
	objectName    string // Human-readable name of the object input (for breadcrumb)
}

// CreateFrame captures the wizard state when suspending for nested bundle creation.
type CreateFrame struct {
	flatBundleCursor       int
	selectedCollIdx        int
	selectedBundleIdx      int
	selectedBundleDefEntry *config.BundleDefinitionEntry
	selectedBundleSource   string
	inputsForm             InputsForm
	parentBundleName       string // name of the bundle being configured (for UI context)
}

// flatBundleEntry maps a row in the flat bundle list back to its collection/bundle origin.
type flatBundleEntry struct {
	collIdx   int
	bundleIdx int
	bundle    *manifest.Bundle
	collName  string
	isLocal   bool
}

// envFilterState represents one valid filter option for env cycling.
type envFilterState struct {
	env     *config.Environment // nil for "without environment"
	envLess bool                // true for the "without environment" filter
	label   string              // display label for breadcrumb (Name)
	shortID string              // short label for help text (ID or "env-less")
}

// InputOption represents a selectable option for select/multiselect inputs.
type InputOption struct {
	Label string
	Value cty.Value
}

// EngineState contains common engine state used throughout all layers.
//
// None of these attributes should change after initialization,
// with the exception of Registry, which is reloaded after each save.
type EngineState struct {
	Context         context.Context
	CLI             commands.CLI
	WorkingDir      string
	Root            *config.Root
	Evalctx         *eval.Context // This is the base evalctx. Should be cloned instead of modifie directly.
	ResolveAPI      resolve.API
	Registry        *config.Registry
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
	selectedEnv *config.Environment

	// Overview state
	focus      FocusArea
	commandIdx int
	commands   []string

	summaryCursor        int                     // Selected row in the session bundles list
	changeLog            []string                // cumulative log of all saved changes across the session (for CLI exit)
	sessionChanges       map[string][]ChangeKind // bundle key → ordered list of change kinds applied this session
	lastSavedKey         string                  // bundle key of the most recently saved change (cleared on next keypress)
	confirmingCreateExit bool                    // true when showing wizard exit confirmation
	createExitConfirmIdx int                     // 0 = Yes, 1 = No

	// Bundle selection state (flat list)
	flatBundles            []flatBundleEntry
	flatBundleCursor       int
	selectedCollIdx        int // Set by selectFlatBundle, used by loadBundleDef
	selectedBundleIdx      int // Set by selectFlatBundle, used by loadBundleDef
	selectedBundleDefEntry *config.BundleDefinitionEntry
	selectedBundleSource   string
	inputsForm             InputsForm
	createEnvCursor        int // Cursor for ViewCreateEnvSelect

	// Bundle reference / nested creation state
	createStack    []CreateFrame // Stack of suspended wizard states
	nestedRefClass string        // When non-empty, we're creating a bundle for this class

	// Object input nested editing state
	objectEditStack []ObjectEditFrame // Stack for nested object input editing

	// Reconfigure state
	reconfigBundles      []*config.Bundle // Filtered bundles for current filter, rebuilt on filter change
	reconfigCursor       int              // Cursor in reconfigBundles
	reconfigBundle       *config.Bundle   // The bundle currently being reconfigured
	reconfigFromOverview bool             // true when reconfig was entered from session panel (skip ViewReconfigSelect on ESC)
	reconfigFilters      []envFilterState // Precomputed valid filter states
	reconfigFilterPos    int              // Current position in reconfigFilters (-1 = all/no filter)

	// Promote state
	promoteBundles    []*config.Bundle      // Filtered bundles for current filter
	promoteTargetEnvs []*config.Environment // Target env per bundle (parallel to promoteBundles)
	promoteCursor     int                   // Cursor in promoteBundles
	promoteBundle     *config.Bundle        // The bundle currently being promoted
	promoteFilters    []envFilterState      // Precomputed valid filter states
	promoteFilterPos  int                   // Current position in promoteFilters (-1 = all/no filter)

	// Transient status
	currentErr       error  // Shown in the overview error area, cleared on next keypress
	ctrlCPending     bool   // true after first ctrl+c press, reset after 1s
	errorDialogTitle string // Title for the error dialog (e.g. "Bundle is not enabled")
	errorDialogText  string // When non-empty, shows a dismissible error dialog overlay

	// Result
	err       error
	cancelled bool
}

const uiWidth = 100         // Default/preferred panel width
const minPanelWidth = 72    // Minimum panel width (fits 80-col terminals)
const minContentHeight = 12 // Minimum visible content lines inside bordered panels

// effectiveContentHeight returns the number of content lines available inside
// bordered panels, scaling with the terminal height. The value is clamped to a
// minimum of minContentHeight so the UI stays usable on small terminals.
func (m Model) effectiveContentHeight() int {
	if m.height <= 0 {
		return minContentHeight
	}
	// Chrome: outer padding(2) + header(1) + help(1) + border(2) + inner padding(2) + margin(2) = 10
	h := m.height - 10
	if h < minContentHeight {
		return minContentHeight
	}
	return h
}

// effectiveInputsPanelHeight returns the Height() budget for input-form panels.
// It equals effectiveContentHeight()+6 so that input-form views fill the same
// terminal space as select-panel views.
func (m Model) effectiveInputsPanelHeight() int {
	return m.effectiveContentHeight() + 6
}

// effectiveWidth returns the effective panel width for views,
// clamped between minPanelWidth and uiWidth*2 based on available terminal space.
func (m Model) effectiveWidth() int {
	if m.width <= 0 {
		return uiWidth
	}
	w := m.width - 6 // outer Padding(1, 2) = 2 left + 2 right + border 1 left + 1 right
	if w < minPanelWidth {
		return minPanelWidth
	}
	if w > uiWidth*2 {
		return uiWidth * 2
	}
	return w
}

// NewModel creates a new prompt model.
func NewModel(est *EngineState) Model {
	var initialViewState ViewState

	if _, err := os.Stat(cliauth.CredentialFile(est.CLIConfig)); err != nil && shouldAskForLogin(est.CLIConfig) {
		initialViewState = ViewCloudLogin
	} else {
		initialViewState = ViewOverview
	}

	return Model{
		EngineState: est,
		viewState:   initialViewState,
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
		m.inputsForm.PanelWidth = m.effectiveWidth()
		m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
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
		m.viewState = ViewOverview
		return m, textarea.Blink

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			// Clear error dialog so it doesn't block the second Ctrl+C
			m.errorDialogTitle = ""
			m.errorDialogText = ""
			if m.ctrlCPending {
				m.cancelled = true
				return m, tea.Quit
			}
			m.ctrlCPending = true
			return m, tea.Tick(time.Second, func(time.Time) tea.Msg {
				return ctrlCResetMsg{}
			})
		}

		// Error dialog intercept — dismiss on any key
		if m.errorDialogText != "" {
			m.errorDialogTitle = ""
			m.errorDialogText = ""
			return m, nil
		}

		switch m.viewState {
		case ViewCloudLogin:
			return m.updateCloudLogin(msg)
		case ViewCreateSelect:
			return m.updateCreateSelect(msg)
		case ViewCreateEnvSelect:
			return m.updateCreateEnvSelect(msg)
		case ViewCreateInput:
			return m.updateCreateInput(msg)
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
		case ViewCreateInput, ViewReconfigInput, ViewPromoteInput:
			var cmd tea.Cmd
			m.inputsForm, cmd = m.inputsForm.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// View renders the current view.
func (m Model) View() string {
	var base string
	switch m.viewState {
	case ViewCloudLogin:
		base = m.renderCloudLoginView()
	case ViewCreateSelect:
		base = m.renderBundleSelectView()
	case ViewCreateEnvSelect:
		base = m.renderCreateEnvSelectView()
	case ViewCreateInput:
		base = m.renderCreateInputView()
	case ViewReconfigSelect:
		base = m.renderReconfigSelectView()
	case ViewReconfigInput:
		base = m.renderReconfigInputView()
	case ViewPromoteSelect:
		base = m.renderPromoteSelectView()
	case ViewPromoteInput:
		base = m.renderPromoteInputView()
	default:
		base = m.renderOverviewView()
	}

	if m.errorDialogText != "" {
		base = m.overlayErrorDialog()
	}
	return base
}

// overlayErrorDialog renders an error dialog centered on the screen.
func (m Model) overlayErrorDialog() string {
	dialogWidth := m.effectiveWidth() - 8
	if dialogWidth < 40 {
		dialogWidth = 40
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorError).
		Padding(1, 2).
		Width(dialogWidth)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorError)
	msgStyle := lipgloss.NewStyle().Foreground(colorText).Width(dialogWidth - 6)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).MarginTop(1)

	dialog := borderStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(m.errorDialogTitle),
			"",
			msgStyle.Render(m.errorDialogText),
			hintStyle.Render("Press any key to dismiss"),
		),
	)

	w := m.width
	h := m.height
	if w == 0 {
		w = 100
	}
	if h == 0 {
		h = 30
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, dialog,
		lipgloss.WithWhitespaceChars(" "),
	)
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
	m.errorDialogTitle = "Error"
	m.errorDialogText = err.Error()
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

// MatchingBundleOptions returns bundles that match the given class ID and environment.
func MatchingBundleOptions(r *config.Registry, classID string, env *config.Environment) []BundleOption {
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
	return options
}

// IsBundleUnique checks that no existing bundle conflicts with the given alias and class.
func IsBundleUnique(r *config.Registry, alias, classID, hostPath string, env *config.Environment) error {
	skipFileExistsCheck := false

	for _, b := range r.Bundles {
		bundleHostPath := b.Info.HostPath()
		if classID == b.DefinitionMetadata.Class && alias == b.Alias {
			if env != nil && b.Environment != nil {
				if env.ID == b.Environment.ID {
					return errors.E("A bundle with alias %q already exists for environment %s at %s", b.Alias, env.ID, bundleHostPath)
				}
				// Same alias+class, but different env. This is ok.
				// We have to assume the file exists already in this case.
				skipFileExistsCheck = true
			} else {
				return errors.E("A bundle with alias %q already exists at %s", b.Alias, bundleHostPath)
			}
		}
	}
	if hostPath != "" && !skipFileExistsCheck {
		_, err := os.Stat(hostPath)
		if err == nil {
			return errors.E("A file already exists at the target output path %s", hostPath)
		}
	}

	return nil
}

type loginSkipFileData struct {
	Timestamp uint64 `json:"timestamp"`
	Version   string `json:"version"`
}

func shouldAskForLogin(clicfg cliconfig.Config) bool {
	loginSkipFile := filepath.Join(clicfg.UserTerramateDir, "login_skip")
	data, err := os.ReadFile(loginSkipFile)
	if err != nil {
		return true
	}
	var skip loginSkipFileData
	if err := json.Unmarshal(data, &skip); err != nil {
		return true
	}
	if skip.Version != terramate.Version() {
		return true
	}
	skippedAt := time.Unix(int64(skip.Timestamp), 0)
	return time.Since(skippedAt) > 7*24*time.Hour
}

func setLoginSkipped(clicfg cliconfig.Config) error {
	skip := loginSkipFileData{
		Timestamp: uint64(time.Now().Unix()),
		Version:   terramate.Version(),
	}
	data, err := json.Marshal(skip)
	if err != nil {
		return errors.E(err, "marshaling login skip data")
	}
	if err := os.MkdirAll(clicfg.UserTerramateDir, 0o700); err != nil {
		return errors.E(err, "creating user terramate dir")
	}
	loginSkipFile := filepath.Join(clicfg.UserTerramateDir, "login_skip")
	return os.WriteFile(loginSkipFile, data, 0o600)
}
