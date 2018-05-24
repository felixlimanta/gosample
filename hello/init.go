package hello

import (
	"context"
	"database/sql"
	"expvar"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/tokopedia/sqlt"
	"gopkg.in/tokopedia/logging.v1"
)

type ServerConfig struct {
	Name string
}

type DatabaseConfig struct {
	Type       string
	Connection string
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type HelloWorldModule struct {
	cfg       *Config
	db        *sqlt.DB
	something string
	stats     *expvar.Int
}

func NewHelloWorldModule() *HelloWorldModule {

	var cfg Config

	ok := logging.ReadModuleConfig(&cfg, "config", "hello") || logging.ReadModuleConfig(&cfg, "files/etc/gosample", "hello")
	if !ok {
		// when the app is run with -e switch, this message will automatically be redirected to the log file specified
		log.Fatalln("failed to read config")
	}

	masterDB := cfg.Database.Connection
	slaveDB := cfg.Database.Connection
	dbConnection := fmt.Sprintf("%s;%s", masterDB, slaveDB)

	db, err := sqlt.Open(cfg.Database.Type, dbConnection)
	if err != nil {
		log.Fatalln("Failed to connect database. Error: ", err.Error())
	}

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("hello init called", cfg.Server.Name)

	return &HelloWorldModule{
		cfg:       &cfg,
		db:        db,
		something: "John Doe",
		stats:     expvar.NewInt("rpsStats"),
	}
}

func (hlm *HelloWorldModule) SayHelloWorld(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), r.URL.Path)
	defer span.Finish()

	hlm.stats.Add(1)
	hlm.someSlowFuncWeWantToTrace(ctx, w)
}

func (hlm *HelloWorldModule) someSlowFuncWeWantToTrace(ctx context.Context, w http.ResponseWriter) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "someSlowFuncWeWantToTrace")
	defer span.Finish()

	w.Write([]byte("Hello " + hlm.something))
}

// Table Description
type Table struct {
	ColumnName    string        `json:"column_name" db:"column_name"`
	DataType      string        `json:"data_type" db:"data_type"`
	CharMaxLength sql.NullInt64 `json:"character_maximum_length" db:"character_maximum_length"`
	IsNullable    bool          `json:"is_nullable" db:"is_nullable"`
}

func (hlm *HelloWorldModule) GetTableDescription(w http.ResponseWriter, r *http.Request) {
	test := []Table{}
	query := `SELECT column_name, data_type, character_maximum_length
			  FROM INFORMATION_SCHEMA.COLUMNS
			  WHERE table_name = 'ws_user';`
	err := hlm.db.Select(&test, query)
	if err != nil {
		log.Println("Error Query Database. Error: ", err.Error())
	}

	result := "List:\n"
	for _, v := range test {
		result += fmt.Sprintf("Column %s: %s(%d) (Nullable: %v)\n", v.ColumnName, v.DataType, v.CharMaxLength, v.IsNullable)
	}

	w.Write([]byte(result))
}
