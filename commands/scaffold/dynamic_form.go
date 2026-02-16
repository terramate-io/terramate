// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scaffold

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// DynamicFormState holds state shared across dynamic form steps.
type DynamicFormState struct {
	NavigatingBack bool
}

// DynamicForm is a tea.Model that chains multiple huh.Form steps.
type DynamicForm struct {
	activeForm *huh.Form

	nextFormFuncs []func(*DynamicFormState) (*huh.Form, error)
	err           error
	state         *DynamicFormState
}

// NewDynamicForm creates a DynamicForm from a sequence of form factory functions.
func NewDynamicForm(formFuncs ...func(*DynamicFormState) (*huh.Form, error)) (DynamicForm, error) {
	st := &DynamicFormState{}

	activeForm, err := formFuncs[0](st)
	if err != nil {
		return DynamicForm{}, err
	}

	return DynamicForm{
		activeForm:    activeForm,
		nextFormFuncs: formFuncs[1:],
		state:         st,
	}, nil
}

// Init implements tea.Model.
func (m DynamicForm) Init() tea.Cmd {
	return m.activeForm.Init()
}

// Update implements tea.Model.
func (m DynamicForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Interrupt
		case "shift+tab":
			m.state.NavigatingBack = true
		default:
			m.state.NavigatingBack = false
		}
	}

	var cmds []tea.Cmd

	form, cmd := m.activeForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.activeForm = f
		cmds = append(cmds, cmd)
	}

	if m.activeForm.State == huh.StateCompleted {
		if len(m.nextFormFuncs) > 0 {
			var err error
			nextForm, err := m.nextFormFuncs[0](m.state)
			if err == nil {
				m.activeForm = nextForm
				m.nextFormFuncs = m.nextFormFuncs[1:]
				cmds = append(cmds, m.activeForm.Init())
			} else {
				m.err = err
				cmds = append(cmds, tea.Quit)
			}
		} else {
			cmds = append(cmds, tea.Quit)
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m DynamicForm) View() string {
	return m.activeForm.View()
}

func (m DynamicForm) Error() error {
	return m.err
}
