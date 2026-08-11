package admin

import (
	"strconv"
	"strings"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func editTitle(item article.Article, isNew bool) string {
	if isNew {
		return "新規記事"
	}
	if item.Title == "" {
		return "記事編集"
	}
	return item.Title
}

func boolAttr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func formatPublishedAt(item article.Article) string {
	if item.PublishedAt.IsZero() {
		return ""
	}
	return item.PublishedAt.Format("2006-01-02 15:04")
}

func joinTopics(topics []string) string {
	return strings.Join(topics, ",")
}
