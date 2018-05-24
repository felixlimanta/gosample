package nsq

import (
	"log"
	"os"

	"github.com/gomodule/redigo/redis"
	"github.com/nsqio/go-nsq"
	logging "gopkg.in/tokopedia/logging.v1"
)

type ServerConfig struct {
	Name string
}

type RedisConfig struct {
	Connection string
}

type NSQConfig struct {
	NSQD     string
	Lookupds string
}

type Config struct {
	Server ServerConfig
	Redis  RedisConfig
	NSQ    NSQConfig
}

type RedisModule struct {
	cfg   *Config
	redis *redis.Pool
}

type NSQModule struct {
	cfg *Config
	q   []*nsq.Consumer
}

func NewNSQModule() *NSQModule {
	var cfg Config

	ok := logging.ReadModuleConfig(&cfg, "config", "nsq") || logging.ReadModuleConfig(&cfg, "files/etc/gosample", "nsq")
	if !ok {
		// when the app is run with -e switch, this message will automatically be redirected to the log file specified
		log.Fatalln("Failed to read NSQ config")
	}

	// this message only shows up if app is run with -debug option, so its great for debugging
	logging.Debug.Println("nsq init called", cfg.Server.Name)

	redisPools := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", cfg.Redis.Connection)
			if err != nil {
				return nil, err
			}
			return conn, err
		},
	}

	rm := &RedisModule{
		cfg:   &cfg,
		redis: redisPools,
	}

	// contohnya: caranya ciptakan nsq consumer
	nsqCfg := nsq.NewConfig()

	q := make([]*nsq.Consumer, 2)

	q[0] = createNewConsumer(nsqCfg, "omae-wa-mou-shindeiru", "count", rm.handler)
	q[0].SetLogger(log.New(os.Stderr, "nsq0:", log.Ltime), nsq.LogLevelError)
	q[0].ConnectToNSQLookupd(cfg.NSQ.Lookupds)

	q[1] = createNewConsumer(nsqCfg, "omae-wa-mou-shindeiru", "count", rm.handler)
	q[1].SetLogger(log.New(os.Stderr, "nsq1:", log.Ltime), nsq.LogLevelError)
	q[1].ConnectToNSQLookupd(cfg.NSQ.Lookupds)

	return &NSQModule{
		cfg: &cfg,
		q:   q,
	}
}

func createNewConsumer(nsqCfg *nsq.Config, topic string, channel string, handler nsq.HandlerFunc) *nsq.Consumer {
	q, err := nsq.NewConsumer(topic, channel, nsqCfg)
	if err != nil {
		log.Fatal("failed to create consumer for ", topic, channel, err)
	}
	q.AddHandler(handler)
	return q
}

func (rm *RedisModule) handler(msg *nsq.Message) error {
	err := rm.incrementRedisKey(string(msg.Body))
	if err != nil {
		return err
	}

	msg.Finish()
	return nil
}

func (rm *RedisModule) incrementRedisKey(key string) error {
	pool := rm.redis.Get()
	_, err := pool.Do("INCR", key)
	if err != nil {
		return err
	}

	return nil
}
