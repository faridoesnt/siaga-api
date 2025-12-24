package datasources

import (
	"fmt"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/contracts"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	zero "github.com/rs/zerolog/log"
)

func Init(config map[string]string) *contracts.Datasources {
	var err error
	var dbWriter *sqlx.DB
	var dbReader *sqlx.DB

	dsWriter, dsReader := parseDs(config)

	if dbWriter, err = sqlx.Connect(config[constants.DbDialeg], dsWriter); err == nil {
		dbWriter.SetConnMaxLifetime(time.Duration(1) * time.Second)
		dbWriter.SetMaxOpenConns(10)
		dbWriter.SetMaxIdleConns(10)

		zero.Log().Msg("Initializing Writer DB: Pass")
	} else {
		zero.Panic().
			Str("Context", "Connecting to Writer DB").
			Err(err).Msg("")
	}

	if dbReader, err = sqlx.Connect(config[constants.DbDialeg], dsReader); err == nil {
		dbReader.SetConnMaxLifetime(time.Duration(1) * time.Second)
		dbReader.SetMaxOpenConns(10)
		dbReader.SetMaxIdleConns(10)

		zero.Log().Msg("Initializing Reader DB: Pass")
	} else {
		zero.Panic().
			Str("Context", "Connecting to Reader DB").
			Err(err).Msg("")
	}

	ds := &contracts.Datasources{
		WriterDB: dbWriter,
		ReaderDB: dbReader,
	}

	return ds
}

func parseDs(config map[string]string) (dsWriter, dsReader string) {
	hostWriter := config[constants.DbHostWriter]
	hostReader := config[constants.DbHostReader]
	port := config[constants.DbPort]
	user := config[constants.DbUser]
	pass := config[constants.DbPass]
	name := config[constants.DbName]

	params := "?parseTime=true&loc=Asia%2FJakarta"
	dsWriter = fmt.Sprintf("%s:%s@(%s:%s)/%s%s", user, pass, hostWriter, port, name, params)
	dsReader = fmt.Sprintf("%s:%s@(%s:%s)/%s%s", user, pass, hostReader, port, name, params)

	return
}
