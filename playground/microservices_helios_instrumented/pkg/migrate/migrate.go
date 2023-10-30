package migrate

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	logger "github.com/helios/go-sdk/proxy-libs/helioslogrus"
	_ "github.com/mattes/migrate/source/file"
)

const (
	MIGRATION_INDICATOR_LEN = 6

	SEPARATOR = "_"
)

func getCounter(folder string) int {
	files, err := os.ReadDir(folder)
	if err != nil {
		logger.Fatalf("Error when read from folder %s, err %w", folder, err)
	}

	lastNum := -1
	for _, file := range files {
		// not directory
		if file.IsDir() {
			continue
		}

		// not has expected suffix
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}

		idPrefix := strings.Split(file.Name(), SEPARATOR)[0]
		id, err := strconv.Atoi(idPrefix)
		if err != nil {
			logger.Fatalf("Error when convert string to number %s, err %w", idPrefix, err)
		}

		if id > lastNum {
			lastNum = id
		}
	}

	return lastNum
}

func New(folder string, name string) {
	newMigrateId := strconv.Itoa(getCounter(folder) + 1)
	newMigrateId = fmt.Sprintf("%s%s",
		strings.Repeat("0", MIGRATION_INDICATOR_LEN-len(newMigrateId)),
		newMigrateId)

	newFileName := fmt.Sprintf("%s%s%s",
		newMigrateId,
		SEPARATOR,
		name,
	)

	err := os.WriteFile(fmt.Sprintf("%s/%s.up.sql", folder, newFileName), []byte{}, 0644)
	if err != nil {
		logger.Fatalf("Cannot create file migration up, err %s", err.Error())
	}

	err = os.WriteFile(fmt.Sprintf("%s/%s.down.sql", folder, newFileName), []byte{}, 0644)
	if err != nil {
		logger.Fatalf("Cannot create file migration down, err %s", err.Error())
	}

	logger.Infof("Done create migration for name %s", name)
}

func Up(dsn string, folder string) {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", folder),
		fmt.Sprintf("mysql://%s", dsn),
	)
	if err != nil {
		logger.Fatalf("Error when create new migration instance, err %s", err.Error())
	}
	err = m.Up()
	if err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			logger.Fatalf("Error when migrate up, err %s", err.Error())
		}
	}

	logger.Info("Done migrate up")
}

func Down(dsn string, folder string, version string) {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", folder),
		fmt.Sprintf("mysql://%s", dsn),
	)
	if err != nil {
		logger.Fatalf("Error when create new migration instance, err %s", err.Error())
	}
	step, err := strconv.Atoi(version)
	if err != nil {
		logger.Fatalf("Error when parse migrate option, err %s", err.Error())
	}
	err = m.Steps(-1 * step) // make sure the step is negative, so the down is performed.
	if err != nil {
		logger.Fatalf("Error when migrate up, err %s", err.Error())
	}

	logger.Info("Done migrate up")
}

func Force(dsn string, folder string, version string) {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", folder),
		fmt.Sprintf("mysql://%s", dsn),
	)
	if err != nil {
		logger.Fatalf("Error when create new migration instance, err %s", err.Error())
	}
	step, err := strconv.Atoi(version)
	if err != nil {
		logger.Fatalf("Error when parse migrate option, err %s", err.Error())
	}
	err = m.Force(step)
	if err != nil {
		logger.Fatalf("Error when migrate up, err %s", err.Error())
	}

	logger.Info("Done migrate up")
}
