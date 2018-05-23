package hello

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/opentracing/opentracing-go"
	"gopkg.in/tokopedia/logging.v1"
)

type ServerConfig struct {
	Name string
}

type RedisConfig struct {
	Connection string
}

type Config struct {
	Server ServerConfig
	Redis  RedisConfig
}

type HelloWorldModule struct {
	cfg       *Config
	redis     *redis.Pool
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

	return &HelloWorldModule{
		cfg:       &cfg,
		redis:     redisPools,
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

func (hlm *HelloWorldModule) SetRedis(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	value := r.FormValue("value")

	result := "Redis SET successful"

	pool := hlm.redis.Get()
	res, err := redis.String(pool.Do("SET", key, value))
	if err != nil {
		result = "Redis SET failed\nError: " + err.Error()
	}

	pool.Do("EXPIRE", key, 10)

	w.Write([]byte(fmt.Sprintf("SET %s: %s\n%s: %s", key, value, result, res)))
}

func (hlm *HelloWorldModule) GetRedis(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")

	result := "Redis GET successful"
	pool := hlm.redis.Get()
	value, err := redis.String(pool.Do("GET", key))
	if err != nil {
		result = "Redis SET failed\nError: " + err.Error()
	}

	w.Write([]byte(fmt.Sprintf("GET %s\n%s: %s", key, result, value)))
}
