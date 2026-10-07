package prompt

import (
	"fmt"
	"strings"
)

// Line asks a question on t and reads the answer as a line of text, the
// terminal being left in its ordinary line-editing mode so the user's own
// backspace, paste and so on work. An empty answer returns def. ok is false
// when the terminal closes before an answer comes.
func Line(t *Terminal, question, def string) (answer string, ok bool) {
	if def != "" {
		fmt.Fprintf(t, "%s [%s] ", question, def)
	} else {
		fmt.Fprintf(t, "%s ", question)
	}
	var b []byte
	for {
		c, ok := t.readByte()
		if !ok {
			return "", false
		}
		if c == '\n' {
			break
		}
		b = append(b, c)
	}
	answer = strings.TrimSpace(string(b))
	if answer == "" {
		return def, true
	}
	return answer, true
}

// YesNo asks a yes/no question on t, with the default shown as a capital
// ("[y/N]"), and keeps asking until the answer is one it understands. ok is
// false when the terminal closes before an answer comes.
func YesNo(t *Terminal, question string, def bool) (yes bool, ok bool) {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	for {
		answer, ok := Line(t, question+" "+hint, "")
		if !ok {
			return false, false
		}
		if parsed, understood := ParseYesNo(answer, def); understood {
			return parsed, true
		}
		fmt.Fprintln(t, "please answer y or n")
	}
}

// ParseYesNo reads an answer to a yes/no question: empty is def, and
// understood is false for anything that is neither a yes nor a no.
func ParseYesNo(answer string, def bool) (yes bool, understood bool) {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "":
		return def, true
	case "y", "yes":
		return true, true
	case "n", "no":
		return false, true
	}
	return false, false
}
