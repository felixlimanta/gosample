package hello

import (
	"context"
	"database/sql"
	"expvar"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/lib/pq"
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
	render    *template.Template
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

	renderingEngine := template.Must(template.ParseGlob("files/var/templates/index.html"))

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("hello init called", cfg.Server.Name)

	return &HelloWorldModule{
		cfg:       &cfg,
		db:        db,
		render:    renderingEngine,
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
	IsNullable    string        `json:"is_nullable" db:"is_nullable"`
}

func (hlm *HelloWorldModule) GetTableDescription(w http.ResponseWriter, r *http.Request) {
	test := []Table{}
	query := `SELECT column_name, data_type, character_maximum_length, is_nullable
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

// Database lookup
type User struct {
	ID             int         `db:"user_id"`
	Name           string      `db:"full_name"`
	MSISDN         string      `db:"msisdn"`
	Email          string      `db:"user_email"`
	BirthTimeRaw   pq.NullTime `db:"birth_date"`
	BirthDate      string
	UserAge        int       `db:"current_age"`
	CreatedTimeRaw time.Time `db:"create_time"`
	CreatedTime    string
	UpdatedTimeRaw pq.NullTime `db:"update_time"`
	UpdatedTime    string
}

func (hlm *HelloWorldModule) Render(w http.ResponseWriter, r *http.Request) {
	visitorCount := 0
	searchCount := 0

	data := map[string]interface{}{
		"users":        hlm.queryDatabase(r.FormValue("q")),
		"visitorCount": visitorCount,
		"searchCount":  searchCount,
	}

	err := hlm.render.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func (hlm *HelloWorldModule) RenderBatch(w http.ResponseWriter, r *http.Request) {
	visitorCount := 0
	searchCount := 0

	data := map[string]interface{}{
		"users":        hlm.queryDatabase(r.FormValue("q")),
		"visitorCount": visitorCount,
		"searchCount":  searchCount,
	}

	err := hlm.render.ExecuteTemplate(w, "batch", data)
	if err != nil {
		panic(err)
	}
}

func (hlm *HelloWorldModule) queryDatabase(name string) []User {
	users := []User{}
	query := ""
	if name == "" {
		query = `
			SELECT user_id, full_name, msisdn, user_email, birth_date,
				COALESCE(EXTRACT(YEAR from AGE(birth_date)), 0) AS current_age,
				create_time, update_time
			FROM ws_user
			ORDER BY full_name ASC
			LIMIT 10;`
	} else {
		query = `
			SELECT user_id, full_name, msisdn, user_email, birth_date,
				COALESCE(EXTRACT(YEAR from AGE(birth_date)), 0) AS current_age,
				create_time, update_time
			FROM ws_user
			WHERE full_name ILIKE '` + name + `%'
			ORDER BY full_name ASC
			LIMIT 10;`
	}

	err := hlm.db.Select(&users, query)
	if err != nil {
		panic(err)
	}

	for id := range users {
		users[id].BirthDate = "-"
		val, _ := users[id].BirthTimeRaw.Value()
		if val != nil {
			users[id].BirthDate = val.(time.Time).Format(time.ANSIC)
		}

		users[id].CreatedTime = users[id].CreatedTimeRaw.Format(time.ANSIC)

		users[id].UpdatedTime = "-"
		val, _ = users[id].UpdatedTimeRaw.Value()
		if val != nil {
			users[id].UpdatedTime = val.(time.Time).Format(time.ANSIC)
		}
	}

	return users
}
