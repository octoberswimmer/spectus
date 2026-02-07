package markdown

import "testing"

func TestToHTMLLink(t *testing.T) {
	input := "Jane St [apparently uses apex](https://example.com)"
	want := `<p style="margin: 0.5rem 0;">Jane St <a href="https://example.com" target="_blank" style="color: var(--primary); text-decoration: underline;">apparently uses apex</a></p>`
	if got := ToHTML(input); got != want {
		t.Fatalf("ToHTML() mismatch\nwant: %s\ngot:  %s", want, got)
	}
}

func TestToHTMLInlineFormatting(t *testing.T) {
	input := "Use **bold** and *italic* and `code`."
	want := `<p style="margin: 0.5rem 0;">Use <strong>bold</strong> and <em>italic</em> and <code style="background: #2d2d2d; color: #f8f8f2; padding: 0.125rem 0.35rem; border-radius: 3px; font-family: 'Consolas', 'Monaco', monospace; font-size: 0.9em;">code</code>.</p>`
	if got := ToHTML(input); got != want {
		t.Fatalf("ToHTML() mismatch\nwant: %s\ngot:  %s", want, got)
	}
}

func TestToHTMLList(t *testing.T) {
	input := "- one\n- two"
	want := `<ul style="margin: 0.5rem 0; padding-left: 1.5rem;">` + "\n" +
		`<li>one</li>` + "\n" +
		`<li>two</li>` + "\n" +
		`</ul>`
	if got := ToHTML(input); got != want {
		t.Fatalf("ToHTML() mismatch\nwant: %s\ngot:  %s", want, got)
	}
}

func TestToHTMLBlockquote(t *testing.T) {
	input := "> quote"
	want := `<blockquote style="margin: 0.5rem 0; padding: 0.5rem 1rem; border-left: 4px solid var(--primary); background: var(--bg-hover); color: var(--text-secondary); font-style: italic;">` + "\n" +
		`<p style="margin: 0.25rem 0;">quote</p>` + "\n" +
		`</blockquote>`
	if got := ToHTML(input); got != want {
		t.Fatalf("ToHTML() mismatch\nwant: %s\ngot:  %s", want, got)
	}
}
