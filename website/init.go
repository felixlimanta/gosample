package hello

import (
	"database/sql"
	"expvar"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/lib/pq"
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

type RedisConfig struct {
	Connection string
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type WebsiteModule struct {
	cfg    *Config
	db     *sqlt.DB
	render *template.Template
	redis  *redis.Pool
	stats  *expvar.Int
}

func NewWebsiteModule() *WebsiteModule {
	var cfg Config

	ok := logging.ReadModuleConfig(&cfg, "config", "website") || logging.ReadModuleConfig(&cfg, "files/etc/gosample", "website")
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

	redisPools := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", cfg.Redis.Connection)
			if err != nil {
				return nil, err
			}
			return conn, err
		},
	}

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("hello init called", cfg.Server.Name)

	return &WebsiteModule{
		cfg:    &cfg,
		db:     db,
		render: renderingEngine,
		redis:  redisPools,
		stats:  expvar.NewInt("rpsStats"),
	}
}

// Table Description
type Table struct {
	ColumnName    string        `json:"column_name" db:"column_name"`
	DataType      string        `json:"data_type" db:"data_type"`
	CharMaxLength sql.NullInt64 `json:"character_maximum_length" db:"character_maximum_length"`
	IsNullable    string        `json:"is_nullable" db:"is_nullable"`
}

func (wm *WebsiteModule) GetTableDescription(w http.ResponseWriter, r *http.Request) {
	test := []Table{}
	query := `SELECT column_name, data_type, character_maximum_length, is_nullable
			  FROM INFORMATION_SCHEMA.COLUMNS
			  WHERE table_name = 'ws_user';`
	err := wm.db.Select(&test, query)
	if err != nil {
		log.Println("Error Query Database. Error: ", err.Error())
	}

	result := "Columns:\n"
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

func (wm *WebsiteModule) Render(w http.ResponseWriter, r *http.Request) {
	err := wm.incrementRedisKey("visitor_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

	visitorCount, err := wm.getRedisKey("visitor_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

	searchCount, err := wm.getRedisKey("search_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

	data := map[string]interface{}{
		"users":        wm.queryDatabase(r.FormValue("q")),
		"visitorCount": visitorCount,
		"searchCount":  searchCount,
	}

	err = wm.render.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func (wm *WebsiteModule) RenderBatch(w http.ResponseWriter, r *http.Request) {
	visitorCount, err := wm.getRedisKey("visitor_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

	searchCount, err := wm.getRedisKey("search_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

	data := map[string]interface{}{
		"users":        wm.queryDatabase(r.FormValue("q")),
		"visitorCount": visitorCount,
		"searchCount":  searchCount,
	}

	err = wm.render.ExecuteTemplate(w, "batch", data)
	if err != nil {
		panic(err)
	}
}

func (wm *WebsiteModule) queryDatabase(name string) []User {
	err := wm.incrementRedisKey("search_count")
	if err != nil {
		log.Fatalln(err.Error())
	}

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

	err = wm.db.Select(&users, query)
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

func (wm *WebsiteModule) incrementRedisKey(key string) error {
	pool := wm.redis.Get()
	_, err := pool.Do("INCR", key)
	if err != nil {
		return err
	}

	return nil
}

func (wm *WebsiteModule) getRedisKey(key string) (int, error) {
	pool := wm.redis.Get()
	val, err := redis.Int(pool.Do("GET", key))
	if err != nil {
		return 0, err
	}

	return val, nil
}
