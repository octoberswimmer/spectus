package markdown

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	codeBlockRe        = regexp.MustCompile("(?s)```([^\\n`]*)\\n?(.*?)```")
	boldSectionRe      = regexp.MustCompile(`\*\*([^*]+)\*\*:`)
	boldRe             = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe           = regexp.MustCompile(`\*([^*]+)\*`)
	inlineCodeRe       = regexp.MustCompile("`([^`]+)`")
	linkRe             = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	codePlaceholderRe  = regexp.MustCompile(`^__CODE_BLOCK_\d+__$`)
)

// ToHTML converts a subset of Markdown to HTML that matches task-manager.html.
func ToHTML(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	html := markdown

	codeBlocks := []string{}
	html = codeBlockRe.ReplaceAllStringFunc(html, func(block string) string {
		parts := codeBlockRe.FindStringSubmatch(block)
		lang := "text"
		if len(parts) > 1 {
			if trimmed := strings.TrimSpace(parts[1]); trimmed != "" {
				lang = trimmed
			}
		}

		code := ""
		if len(parts) > 2 {
			code = parts[2]
		}
		code = strings.TrimSuffix(code, "\n")
		escapedCode := escapeHTML(code)

		codeBlock := fmt.Sprintf(
			`<div style="margin: 1rem 0;"><div style="background: #1a1a1a; color: #888; padding: 0.25rem 0.5rem; border-radius: 6px 6px 0 0; font-size: 0.75rem; font-family: 'Consolas', 'Monaco', monospace;">%s</div><pre style="margin: 0; border-radius: 0 0 6px 6px;"><code>%s</code></pre></div>`,
			lang,
			escapedCode,
		)

		placeholder := fmt.Sprintf("\n__CODE_BLOCK_%d__\n", len(codeBlocks))
		codeBlocks = append(codeBlocks, codeBlock)
		return placeholder
	})

	html = escapeHTML(html)

	html = boldSectionRe.ReplaceAllString(html, `<strong style="color: var(--primary); display: block; margin-top: 1rem; margin-bottom: 0.5rem;">$1:</strong>`)
	html = boldRe.ReplaceAllString(html, `<strong>$1</strong>`)
	html = italicRe.ReplaceAllString(html, `<em>$1</em>`)
	html = inlineCodeRe.ReplaceAllString(html, `<code style="background: #2d2d2d; color: #f8f8f2; padding: 0.125rem 0.35rem; border-radius: 3px; font-family: 'Consolas', 'Monaco', monospace; font-size: 0.9em;">$1</code>`)
	html = linkRe.ReplaceAllString(html, `<a href="$2" target="_blank" style="color: var(--primary); text-decoration: underline;">$1</a>`)

	lines := strings.Split(html, "\n")
	result := make([]string, 0, len(lines))
	inList := false
	inBlockquote := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || codePlaceholderRe.MatchString(trimmed) {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			if inBlockquote {
				result = append(result, "</blockquote>")
				inBlockquote = false
			}
			if codePlaceholderRe.MatchString(trimmed) {
				result = append(result, trimmed)
			} else if trimmed == "" && !inList && !inBlockquote {
				result = append(result, "")
			}
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			if inBlockquote {
				result = append(result, "</blockquote>")
				inBlockquote = false
			}
			if !inList {
				result = append(result, `<ul style="margin: 0.5rem 0; padding-left: 1.5rem;">`)
				inList = true
			}
			result = append(result, "<li>"+strings.TrimSpace(trimmed[2:])+"</li>")
			continue
		}

		if strings.HasPrefix(trimmed, "&gt; ") || trimmed == "&gt;" {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			if !inBlockquote {
				result = append(result, `<blockquote style="margin: 0.5rem 0; padding: 0.5rem 1rem; border-left: 4px solid var(--primary); background: var(--bg-hover); color: var(--text-secondary); font-style: italic;">`)
				inBlockquote = true
			}
			quoteContent := strings.TrimSpace(strings.TrimPrefix(trimmed, "&gt;"))
			if quoteContent != "" {
				result = append(result, `<p style="margin: 0.25rem 0;">`+quoteContent+`</p>`)
			}
			continue
		}

		if inList {
			result = append(result, "</ul>")
			inList = false
		}
		if inBlockquote {
			result = append(result, "</blockquote>")
			inBlockquote = false
		}
		if trimmed != "" {
			result = append(result, `<p style="margin: 0.5rem 0;">`+line+`</p>`)
		}
	}

	if inList {
		result = append(result, "</ul>")
	}
	if inBlockquote {
		result = append(result, "</blockquote>")
	}

	html = strings.Join(result, "\n")

	for i, block := range codeBlocks {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
		html = strings.ReplaceAll(html, placeholder, block)
	}

	return html
}

func escapeHTML(input string) string {
	if input == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(input)
}
