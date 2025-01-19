package database

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "github.com/yerTools/imapbackup/src/go/database/migrations"
)

func Init(app *pocketbase.PocketBase, isGoRun bool) {
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Dashboard
		// (the isGoRun check is to enable it only during development)
		Automigrate: isGoRun,
		Dir:         "./src/go/database/migrations",
	})
}
