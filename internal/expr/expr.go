package expr

import "regexp"

var identRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

type Identifier struct {
	Name        string
	Start       int
	End         int
	TimeIndexed bool
	Function    bool
}

func FindIdentifiers(text string) []Identifier {
	matches := identRE.FindAllStringIndex(text, -1)
	out := make([]Identifier, 0, len(matches))

	for _, match := range matches {
		ident := Identifier{
			Name:  text[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		}

		next := skipSpaces(text, ident.End)
		if next < len(text) && text[next] == '(' {
			afterParen := skipSpaces(text, next+1)
			if afterParen < len(text) && text[afterParen] == 't' {
				ident.TimeIndexed = true
			} else {
				ident.Function = true
			}
		}

		out = append(out, ident)
	}

	return out
}

func skipSpaces(text string, i int) int {
	for i < len(text) {
		switch text[i] {
		case ' ', '\t':
			i++
		default:
			return i
		}
	}

	return i
}
