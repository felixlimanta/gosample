package hello

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"gopkg.in/tokopedia/logging.v1"
)

type ServerConfig struct {
	Name string
}

type Config struct {
	Server ServerConfig
}

type HelloWorldModule struct {
	cfg       *Config
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

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("hello init called", cfg.Server.Name)

	return &HelloWorldModule{
		cfg:       &cfg,
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

	w.Write([]byte("喜べ " + hlm.something))
}

// TRAINING
func (hlm *HelloWorldModule) Test(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		name = "少年"
	}

	w.Write([]byte(fmt.Sprintf("喜べ、%s！君のねがうぃやようやくかなう", name)))
}
