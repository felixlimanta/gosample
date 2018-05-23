package hello

import (
	"context"
	"encoding/json"
	"expvar"
	"log"
	"net/http"

	"github.com/nsqio/go-nsq"
	"github.com/opentracing/opentracing-go"
	"gopkg.in/tokopedia/logging.v1"
)

type ServerConfig struct {
	Name string
}

type NSQConfig struct {
	NSQD     string
	Lookupds string
}

type Config struct {
	Server ServerConfig
	NSQ    NSQConfig
}

type HelloWorldModule struct {
	cfg       *Config
	nsq       *nsq.Producer
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

	nsqProducer, e := nsq.NewProducer(cfg.NSQ.NSQD, nsq.NewConfig())
	if e != nil {
		log.Fatalln("Failed to create new producer: ", e.Error())
	}

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("hello init called", cfg.Server.Name)

	return &HelloWorldModule{
		cfg:       &cfg,
		nsq:       nsqProducer,
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

func (hlm *HelloWorldModule) PublishNSQ(w http.ResponseWriter, r *http.Request) {
	hlm.stats.Add(1)

	name := r.FormValue("name")
	if name == "" {
		name = "お前"
	}

	message := r.FormValue("message")
	if message == "" {
		message = "　はもう死んでいる"
	}

	data := map[string]string{
		"name":    name,
		"message": message,
	}

	nsqMessage, e := json.Marshal(data)
	if e != nil {
		panic(e)
	}

	result := "Push NSQ Success"

	e = hlm.nsq.Publish("nsq-training", nsqMessage)
	if e != nil {
		log.Printf("Failed to publish NSQ\n")
		result = "Push NSQ Failure"
	}

	w.Write([]byte(result))
}
