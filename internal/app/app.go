package app

import (
	"fmt"
	"log/slog"

	"github.com/SouichiroTsujimoto/unigo-template/internal/db"
	"github.com/SouichiroTsujimoto/unigo-template/internal/feature/account"
	"github.com/SouichiroTsujimoto/unigo-template/internal/httpserver"
	"github.com/SouichiroTsujimoto/unigo-template/internal/terminal"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web/about"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web/home"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web/islands"
	"github.com/SouichiroTsujimoto/unigo-template/static"
)

type Config struct {
	Address   string
	DB        db.Config
	Domains   []string
	ACMEEmail string
	Version   string
	Banner    string
}

func Run(config Config) error {
	log := terminal.NewLogger()
	slog.SetDefault(log)

	database, err := db.Open(config.DB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	accounts := account.New(database)
	homeHandler := home.New(accounts, log)
	aboutHandler := about.New(log)
	handler := web.New(homeHandler, aboutHandler, static.FS(), islands.FS())

	return httpserver.Run(handler, httpserver.Config{
		Address:   config.Address,
		Domains:   config.Domains,
		ACMEEmail: config.ACMEEmail,
		DBPath:    config.DB.WithDefaults().Label(),
		Version:   config.Version,
		Banner:    config.Banner,
	}, log)
}
