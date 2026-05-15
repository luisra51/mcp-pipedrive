package pipedrive

import "github.com/jaytaylor/html2text"

// HTMLToText converts an HTML email body to plain text via
// jaytaylor/html2text. Links render as "label ( url )"; <script>/<style>
// contents are dropped; entities are decoded; whitespace is collapsed.
//
// PrettyTables is intentionally OFF — HTML emails use tables for layout
// and ASCII-bordered tables blow output 10–15x larger than the input.
// On conversion error we return the raw HTML so the caller still sees
// something rather than nothing.
func HTMLToText(s string) string {
	if s == "" {
		return ""
	}
	out, err := html2text.FromString(s, html2text.Options{})
	if err != nil {
		return s
	}
	return out
}

// Body-format values accepted by the mailbox tools when projecting a
// MailMessage to its agent-facing shape.
const (
	BodyFormatText = "text" // HTML-stripped plain text (default)
	BodyFormatHTML = "html" // raw HTML as Pipedrive returned it
	BodyFormatNone = "none" // omit body entirely
)

// Thread-projection values accepted by pipedrive.mail.threads.list.
const (
	MailFieldsCompact = "compact"
	MailFieldsFull    = "full"
)

// MailCompactSnippetMax caps the per-thread snippet length under
// MailFieldsCompact. Pipedrive snippets are ~240 chars; 120 is enough
// at a glance and halves typical bytes per thread.
const MailCompactSnippetMax = 120
