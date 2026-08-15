// Package terminal contains output-boundary helpers for terminal text.
package terminal

import (
	"fmt"
	"strings"
	"unicode"
)

// EscapeControls converts control and Unicode format characters into visible
// ASCII escapes. It is intended for text sent to a terminal, not for data
// serialization or process arguments.
func EscapeControls(value string) string {
	var escaped strings.Builder
	for _, r := range value {
		if !unsafeRune(r) {
			escaped.WriteRune(r)
			continue
		}
		switch {
		case r <= 0xff:
			escaped.WriteString(fmt.Sprintf("\\x%02X", r))
		case r <= 0xffff:
			escaped.WriteString(fmt.Sprintf("\\u%04X", r))
		default:
			escaped.WriteString(fmt.Sprintf("\\U%08X", r))
		}
	}
	return escaped.String()
}

func unsafeRune(r rune) bool {
	return r == 0x7f || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}
