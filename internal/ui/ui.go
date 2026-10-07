// Package ui is how goscaffold asks the user things: on the terminal when
// there is one and it's allowed, otherwise by taking each question's default.
package ui

import (
	"errors"

	"goscaffold/internal/prompt"
)

// ErrCancelled is returned when the user backs out of a question.
var ErrCancelled = errors.New("cancelled")

// UI asks questions.
type UI interface {
	// Interactive reports whether questions really reach a person.
	Interactive() bool
	// Select asks the user to pick one of options; def is the index
	// highlighted first.
	Select(question string, options []string, def int) (int, error)
	// Input asks for a line of text; def is returned for an empty answer.
	Input(question, def string) (string, error)
	// Confirm asks a yes/no question.
	Confirm(question string, def bool) (bool, error)
}

// Auto is a terminal UI when interactive is allowed and a terminal can be
// opened, and otherwise a UI that answers every question with its default.
// The returned func releases the terminal.
func Auto(allowInteractive bool) (UI, func()) {
	if allowInteractive {
		if t, ok := prompt.Open(); ok {
			return &terminalUI{t: t}, t.Close
		}
	}
	return Defaults{}, func() {}
}

// Defaults answers every question with its default.
type Defaults struct{}

func (Defaults) Interactive() bool { return false }

func (Defaults) Select(_ string, _ []string, def int) (int, error) { return def, nil }

func (Defaults) Input(_, def string) (string, error) { return def, nil }

func (Defaults) Confirm(_ string, def bool) (bool, error) { return def, nil }

type terminalUI struct {
	t *prompt.Terminal
}

func (u *terminalUI) Interactive() bool { return true }

func (u *terminalUI) Select(question string, options []string, def int) (int, error) {
	u.t.Write([]byte(question + "\r\n"))
	i, ok := prompt.Select(u.t, options, def)
	if !ok {
		return 0, ErrCancelled
	}
	return i, nil
}

func (u *terminalUI) Input(question, def string) (string, error) {
	answer, ok := prompt.Line(u.t, question, def)
	if !ok {
		return "", ErrCancelled
	}
	return answer, nil
}

func (u *terminalUI) Confirm(question string, def bool) (bool, error) {
	yes, ok := prompt.YesNo(u.t, question, def)
	if !ok {
		return false, ErrCancelled
	}
	return yes, nil
}
