package tui

import "charm.land/bubbles/v2/key"

var (
	quitKey    = key.NewBinding(key.WithKeys("ctrl+c"))
	connectKey = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "connect"),
	)
	addKey = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "add"),
	)
	editKey = key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "edit"),
	)
	duplicateKey = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "duplicate"),
	)
	deleteKey = key.NewBinding(
		key.WithKeys("delete", "backspace"),
		key.WithHelp("del", "delete"),
	)
	enterKey = key.NewBinding(key.WithKeys("enter"))
	skipKey  = key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "skip this section"),
	)
)
