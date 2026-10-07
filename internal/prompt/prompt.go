// Package prompt provides small interactive terminal prompts for goscaffold's
// command line — an arrow-key list picker in the style of inquirer, plus
// plain text and yes/no questions.
package prompt

import (
	"fmt"
	"strconv"
	"strings"
)

// Select renders labels as an arrow-key-navigable list on rw — which must be
// both readable and writable and refer to a real terminal, since it is put
// into raw mode for the duration of the call — and returns the chosen
// index. def is the index highlighted first. Up/Down (and k/j) move the
// selection, enter confirms, q/Esc/Ctrl-C cancel (ok=false).
//
// The caller is expected to have already printed the question above the
// list; Select only draws the options and collapses them to the chosen
// answer once the user is done.
func Select(rw *Terminal, labels []string, def int) (choice int, ok bool) {
	if len(labels) == 0 {
		return 0, false
	}
	f, err := openFrame(rw)
	if err != nil {
		return 0, false
	}
	defer f.close()

	sel := def
	if sel < 0 || sel >= len(labels) {
		sel = 0
	}
	f.draw(selectLines(labels, sel))

	for {
		key, ok := rw.readByte()
		if !ok {
			f.clear()
			return 0, false
		}
		switch key {
		case 3, 'q', 'Q': // Ctrl-C, q
			f.clear()
			return 0, false
		case '\r', '\n':
			f.clear()
			fmt.Fprintf(rw, "  \x1b[32m✔\x1b[0m %s\r\n", labels[sel])
			return sel, true
		case 'k':
			if sel > 0 {
				sel--
			}
		case 'j':
			if sel < len(labels)-1 {
				sel++
			}
		case 0x1b:
			switch readEscape(rw) {
			case escUp:
				if sel > 0 {
					sel--
				}
			case escDown:
				if sel < len(labels)-1 {
					sel++
				}
			case escBare:
				f.clear()
				return 0, false
			default:
				continue
			}
		default:
			continue
		}
		f.draw(selectLines(labels, sel))
	}
}

type escKey int

const (
	escOther escKey = iota
	escUp
	escDown
	escShiftUp
	escShiftDown
	escBare
)

// escapeWait is how long readEscape waits for each byte of the rest of an
// arrow-key sequence before giving up and treating the ESC as a bare
// Escape keypress. Real terminals emit the whole sequence in one burst, so
// this only ever matters for a genuine standalone Escape.
const escapeWait = 30 // milliseconds

// readEscape reads what follows an already-consumed ESC byte: a bare
// Escape keypress, a plain arrow ("ESC [ A/B"), or an arrow held with a
// modifier key ("ESC [ 1 ; <modifier> A/B", xterm's scheme for Shift,
// Alt, Ctrl and combinations of them on a cursor key).
func readEscape(rw *Terminal) escKey {
	b1, ok := rw.pollByte()
	if !ok || b1 != '[' {
		return escBare
	}
	var params []byte
	var final byte
	for {
		b, ok := rw.pollByte()
		if !ok {
			return escBare
		}
		if (b >= '0' && b <= '9') || b == ';' {
			params = append(params, b)
			continue
		}
		final = b
		break
	}
	shift := hasShiftModifier(params)
	switch final {
	case 'A':
		if shift {
			return escShiftUp
		}
		return escUp
	case 'B':
		if shift {
			return escShiftDown
		}
		return escDown
	default:
		return escOther
	}
}

// hasShiftModifier reports whether a CSI sequence's parameter bytes encode
// the Shift modifier, alone or combined with another (xterm's "1;<mod>"
// scheme, where <mod>-1 is a bitmask with bit 0 set for Shift).
func hasShiftModifier(params []byte) bool {
	parts := strings.Split(string(params), ";")
	if len(parts) < 2 {
		return false
	}
	mod, err := strconv.Atoi(parts[1])
	if err != nil || mod < 2 {
		return false
	}
	return (mod-1)&1 == 1
}

// listLines renders labels as a list with the one at cursor highlighted —
// prefix(i) goes before label i, a checkbox say — then hint.
func listLines(labels []string, cursor int, prefix func(i int) string, hint string) []string {
	lines := make([]string, 0, len(labels)+1)
	for i, label := range labels {
		if i == cursor {
			lines = append(lines, "\x1b[36m❯ "+prefix(i)+label+"\x1b[0m")
		} else {
			lines = append(lines, "  "+prefix(i)+label)
		}
	}
	return append(lines, "\x1b[2m"+hint+"\x1b[0m")
}

func noPrefix(int) string { return "" }

func selectLines(labels []string, sel int) []string {
	return listLines(labels, sel, noPrefix, "(↑/↓ to move, enter to select, q to quit)")
}
