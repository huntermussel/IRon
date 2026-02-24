package emmetbridge

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func looksLikeHTML(s string) bool {
	// Simple heuristic: has angle brackets.
	// We rely on htmlToEmmet failing if it's not valid.
	return strings.Contains(s, "<") && strings.Contains(s, ">")
}

func looksLikeEmmet(s string) bool {
	if strings.Contains(s, "<") {
		return false
	}
	return strings.Contains(s, ">") || strings.Contains(s, "+")
}

// htmlToEmmet: lightweight converter; handles nested elements and simple sibling groups.
func htmlToEmmet(src string) string {
	node, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return ""
	}
	var walk func(*html.Node) string

	walk = func(n *html.Node) string {
		if n.Type != html.ElementNode {
			return ""
		}
		var parts []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				parts = append(parts, walk(c))
			}
		}
		child := strings.Join(filterEmpty(parts), "+")
		if child != "" {
			return n.Data + ">" + child
		}
		return n.Data
	}

	var outParts []string
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			outParts = append(outParts, walk(c))
		}
	}
	return strings.Join(filterEmpty(outParts), "+")
}

// emmetToHTML: very small subset parser (>, +, *N, {text}); builds minimal HTML.
func emmetToHTML(em string) string {
	tokens := strings.FieldsFunc(em, func(r rune) bool { return r == ' ' || r == '\n' })
	if len(tokens) == 0 {
		return ""
	}
	var out strings.Builder
	for i, tok := range tokens {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(expandEmmet(tok))
	}
	return out.String()
}

func expandEmmet(tok string) string {
	// handle multiplier
	mult := 1
	if idx := strings.Index(tok, "*"); idx > 0 {
		multStr := tok[idx+1:]
		tok = tok[:idx]
		if n, err := strconv.Atoi(multStr); err == nil && n > 0 {
			mult = n
		}
	}
	segments := strings.Split(tok, ">")
	html := expandSegment(segments[0])
	for i := 1; i < len(segments); i++ {
		child := expandSegment(segments[i])
		html = wrap(html, child)
	}
	if mult > 1 {
		return strings.Repeat(html, mult)
	}
	return html
}

func expandSegment(seg string) string {
	// handle text {..}
	text := ""
	if l := strings.Index(seg, "{"); l >= 0 && strings.HasSuffix(seg, "}") {
		text = seg[l+1 : len(seg)-1]
		seg = seg[:l]
	}
	if seg == "" {
		return text
	}
	if text != "" {
		return "<" + seg + ">" + text + "</" + seg + ">"
	}
	return "<" + seg + "></" + seg + ">"
}

func wrap(parentHTML, childHTML string) string {
	if parentHTML == "" {
		return childHTML
	}
	// place child inside last closing tag
	closing := strings.LastIndex(parentHTML, "</")
	if closing == -1 {
		return parentHTML + childHTML
	}
	return parentHTML[:closing] + childHTML + parentHTML[closing:]
}

func filterEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
