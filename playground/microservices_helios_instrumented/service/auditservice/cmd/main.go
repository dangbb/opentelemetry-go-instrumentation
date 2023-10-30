package main

import (
	"github.com/alecthomas/kong"
	_ "github.com/go-sql-driver/mysql"
	logrus "github.com/helios/go-sdk/proxy-libs/helioslogrus"
	"microservice/config"
	"microservice/pkg/migrate"
	"microservice/pkg/trace"
	"microservice/service/auditservice/serve"
)

func main() {
	trace.InitTrace("audit")
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Infof("Start audit")

	cliConfig := config.Config{}
	kongCtx := kong.Parse(&cliConfig)

	switch kongCtx.Command() {
	case "server":
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
			migrate.Up("username:password@tcp(localhost:3320)/dbname?charset=utf8mb4&parseTime=True",
				"/home/dangbb/dangnh-opentelemetry-go-instrumentation/playground/microservices/service/auditservice/migration")
		}
	}
}
