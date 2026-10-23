package subscription

import (
	"regexp"
	"strings"
	"unicode"
)

var splitIPVersion = regexp.MustCompile(`(?i)(?:IPv|V)[46]`)

func splitProfileName(up, down string) string {
	up, down = strings.TrimSpace(up), strings.TrimSpace(down)
	if up == down {
		return up
	}
	a, b := splitIPVersion.FindAllStringIndex(up, -1), splitIPVersion.FindAllStringIndex(down, -1)
	if len(a) == 1 && len(b) == 1 {
		left, right := up[a[0][0]:a[0][1]], down[b[0][0]:b[0][1]]
		if left[len(left)-1] != right[len(right)-1] && strings.EqualFold(up[:a[0][0]], down[:b[0][0]]) && strings.EqualFold(up[a[0][1]:], down[b[0][1]:]) {
			family := "V4V6"
			if strings.HasPrefix(strings.ToLower(left), "ipv") {
				family = "IPv4IPv6"
			}
			return up[:a[0][0]] + family + up[a[0][1]:]
		}
	}
	x, y := []rune(up), []rune(down)
	prefix := 0
	for prefix < len(x) && prefix < len(y) && x[prefix] == y[prefix] {
		prefix++
	}
	// Share whole terms, preserving the original separators and typography.
	for prefix > 0 && !splitNameSeparator(x[prefix-1]) {
		prefix--
	}
	suffix := 0
	for suffix < len(x)-prefix && suffix < len(y)-prefix && x[len(x)-1-suffix] == y[len(y)-1-suffix] {
		suffix++
	}
	for suffix > 0 && !splitNameSeparator(x[len(x)-suffix]) {
		suffix--
	}
	if prefix > 0 || suffix > 0 {
		a, b := string(x[prefix:len(x)-suffix]), string(y[prefix:len(y)-suffix])
		if a != "" && b != "" {
			return string(x[:prefix]) + a + " / " + b + string(x[len(x)-suffix:])
		}
	}
	return up + " ↑ / " + down + " ↓"
}
func splitNameSeparator(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("-_/|·", r) }
