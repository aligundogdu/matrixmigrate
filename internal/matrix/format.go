package matrix

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// mentionRe matches @username patterns in Mattermost messages.
// Mattermost usernames: letters, digits, dots, dashes, underscores.
var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9._\-]+)`)

// FormatMessage converts a Mattermost message to Matrix HTML format.
//
// It renders Markdown to HTML and replaces @username mentions with Matrix
// user pills (<a href="https://matrix.to/#/MXID">@username</a>).
//
// usernameToMxID maps Mattermost usernames to their full Matrix user IDs
// (e.g. "@alice:example.com"). Mentions for unknown usernames are left as-is.
//
// Returns (body, formattedBody). body is the original plain-text message.
// formattedBody is the HTML version; it is empty when message is empty.
func FormatMessage(message string, usernameToMxID map[string]string) (string, string) {
	if message == "" {
		return "", ""
	}

	// Step 1: replace known @mentions with stable placeholder tokens so the
	// Markdown parser cannot split or escape them.
	type pill struct{ token, html string }
	var pills []pill

	processed := mentionRe.ReplaceAllStringFunc(message, func(m string) string {
		username := m[1:] // strip leading @
		mxID, ok := usernameToMxID[username]
		if !ok {
			return m
		}
		token := fmt.Sprintf("MMXPILL%dXMMX", len(pills))
		pills = append(pills, pill{
			token: token,
			html:  fmt.Sprintf(`<a href="https://matrix.to/#/%s">%s</a>`, mxID, m),
		})
		return token
	})

	// Step 2: Markdown → HTML.
	ext := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(ext)
	opts := mdhtml.RendererOptions{Flags: mdhtml.CommonFlags | mdhtml.NoreferrerLinks}
	r := mdhtml.NewRenderer(opts)
	htmlBytes := markdown.ToHTML([]byte(processed), p, r)
	htmlStr := strings.TrimSpace(string(htmlBytes))

	// Step 3: restore mention pills.
	for _, pl := range pills {
		htmlStr = strings.ReplaceAll(htmlStr, pl.token, pl.html)
	}

	return message, htmlStr
}
