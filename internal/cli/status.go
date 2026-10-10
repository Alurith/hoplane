package cli

import (
	"io"

	"github.com/charmbracelet/x/ansi"
)

const programStatusApp = "hoplane"

func (s *commandState) reportProgramStatus(state ansi.ProgramState, title string) {
	status := ansi.SetProgramStatus(ansi.ProgramStatus{
		State: state,
		App:   programStatusApp,
		Title: title,
	})
	if status == "" {
		return
	}
	// Status reporting is advisory and must not change the connection result.
	_, _ = io.WriteString(s.dependencies.Output, status)
}
