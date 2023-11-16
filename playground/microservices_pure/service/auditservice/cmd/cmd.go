package main

import (
	"github.com/alecthomas/kong"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"microservice/config"
	"microservice/pkg/migrate"
	"microservice/service/auditservice/serve"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Infof("Start audit")

	cliConfig := config.Config{}
	kongCtx := kong.Parse(&cliConfig)

	switch kongCtx.Command() {
	case "server":
		migrate.Up(cliConfig.MySqlConfig.GetDsn(),
			cliConfig.MigrationFolder)

		serve.RunAuditServer(cliConfig)
	case "migrate <command> <option>":
		switch cliConfig.Migrate.Command {
		case "down":
			migrate.Down(cliConfig.MySqlConfig.GetDsn(), cliConfig.MigrationFolder, cliConfig.Migrate.Option)
		case "force":
			migrate.Force(cliConfig.MySqlConfig.GetDsn(), cliConfig.MigrationFolder, cliConfig.Migrate.Option)
		case "create":
			migrate.New(cliConfig.MigrationFolder, cliConfig.Migrate.Option)
		}
	case "migrate <command>":
		switch cliConfig.Migrate.Command {
		case "up":
			migrate.Up(cliConfig.MySqlConfig.GetDsn(),
				cliConfig.MigrationFolder)
		}
	}
}
