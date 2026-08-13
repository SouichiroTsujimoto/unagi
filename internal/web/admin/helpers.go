package admin

import (
	"strconv"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func statusLabel(item article.Article) string {
	if item.Published {
		return "公開"
	}
	return "下書き"
}
