package article

import (
	"fmt"
	"strings"
	"time"
)

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// Parse reads Zenn-compatible frontmatter and body from a Markdown file.
func Parse(slug string, data []byte) (Article, error) {
	matches := frontmatterRE.FindSubmatch(data)
	if matches == nil {
		return Article{}, fmt.Errorf("missing YAML frontmatter")
	}

	meta, err := parseFrontmatter(string(matches[1]))
	if err != nil {
		return Article{}, err
	}

	title := strings.TrimSpace(meta["title"])
	if title == "" {
		return Article{}, fmt.Errorf("title is required")
	}

	articleType := strings.TrimSpace(meta["type"])
	if articleType == "" {
		articleType = "tech"
	}
	switch articleType {
	case "tech", "idea":
	default:
		return Article{}, fmt.Errorf("invalid type %q", articleType)
	}

	topics, err := parseTopics(meta["topics"])
	if err != nil {
		return Article{}, err
	}

	return Article{
		Slug:   slug,
		Title:  title,
		Emoji:  strings.TrimSpace(meta["emoji"]),
		Type:   articleType,
		Topics: topics,
		BodyMD: strings.TrimSpace(string(matches[2])) + "\n",
	}, nil
}

func parseFrontmatter(raw string) (map[string]string, error) {
	meta := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid frontmatter line %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "topics" {
			if value == "" || value == "[]" {
				meta[key] = "[]"
				continue
			}
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				meta[key] = value
				continue
			}
			return nil, fmt.Errorf("topics must be a YAML array")
		}
		meta[key] = unquote(value)
	}
	return meta, nil
}

func parseTopics(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("topics must be a YAML array")
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return nil, nil
	}
	parts := splitCSVRespectingQuotes(inner)
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		topic := strings.TrimSpace(unquote(strings.TrimSpace(part)))
		if topic == "" {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		out = append(out, topic)
	}
	if len(out) > 5 {
		return nil, fmt.Errorf("topics: at most 5 allowed")
	}
	return out, nil
}

func splitCSVRespectingQuotes(s string) []string {
	var parts []string
	var b strings.Builder
	inQuote := false
	quote := rune(0)
	for _, r := range s {
		switch {
		case (r == '"' || r == '\'') && !inQuote:
			inQuote = true
			quote = r
			b.WriteRune(r)
		case inQuote && r == quote:
			inQuote = false
			b.WriteRune(r)
		case r == ',' && !inQuote:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	parts = append(parts, b.String())
	return parts
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
