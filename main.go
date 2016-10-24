package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/RobinUS2/golang-moving-average"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errgo"
)

type config struct {
	host     string
	requests int
	sleep    time.Duration
	protocol string
}

func main() {
	var host = flag.String("host", "ip.thaller.ws", "the host to connect to")
	var requests = flag.Int("requests", 10, "the number of requests to send")
	var loglevel = flag.String("loglevel", "info", "the loglevel for the logger")
	var sleep = flag.Duration("sleep", 1000*time.Millisecond, "sleep duration between requests")
	flag.Parse()

	level, err := log.ParseLevel(*loglevel)
	if err != nil {
		log.Fatal(errgo.Notef(err, "can not parse loglevel from flag"))
	}
	log.SetLevel(level)

	conf := config{
		host:     *host,
		requests: *requests,
		sleep:    *sleep,
		protocol: "http",
	}

	fmt.Println("http")
	run_default(conf)
	run_no_keepalive(conf)

	fmt.Println("https")
	conf.protocol = "https"
	run_default(conf)
	run_no_keepalive(conf)
	run_no_session_resume(conf)
	run_no_session_resume_and_no_keepalive(conf)
}

func run_no_session_resume_and_no_keepalive(conf config) {
	fmt.Println("no session resume and no keepalive")
	tr := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			SessionTicketsDisabled: true,
			ClientSessionCache:     new(noCache),
		},
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
	}

	run(client, conf, "run_no_session_resume_and_no_keepalive")
}

func run_no_keepalive(conf config) {
	fmt.Println("no keepalive")
	tr := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(1),
		},
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
	}

	run(client, conf, "run_no_keepalive")
}

func run_no_session_resume(conf config) {
	fmt.Println("no session resume")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			SessionTicketsDisabled: true,
			ClientSessionCache:     new(noCache),
		},
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
	}
	run(client, conf, "run_no_session_resume")
}

func run_default(conf config) {
	fmt.Println("default")
	tr := &http.Transport{
		DisableKeepAlives: false,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(1),
		},
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
	}

	run(client, conf, "run_default")
}

func run(client *http.Client, conf config, suffix string) {
	log.Debug("config: ", conf)

	var durations []time.Duration
	var duration time.Duration

	req, _ := http.NewRequest("HEAD", conf.protocol+`://`+conf.host, nil)
	req.Header.Set("User-Agent", "SSLTest-"+suffix)

	for i := 0; i < conf.requests; i++ {
		start := time.Now()
		res, err := client.Do(req)
		duration = time.Since(start)
		if err != nil {
			log.Fatal(err)
		}

		// Read everything from the response body to make sure that keepalive is
		// working see http://stackoverflow.com/a/17953506
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()

		durations = append(durations, duration)
		log.Debug("count: ", i)
		log.Debug("duration: ", duration)

		log.Debug("sleeping for ", conf.sleep)
		time.Sleep(conf.sleep)
	}

	ma := movingaverage.New(len(durations))
	for _, duration := range durations {
		ma.Add(float64(duration))
	}
	avgduration := time.Duration(ma.Avg())
	fmt.Println("avg duration: ", avgduration)
}

type noCache struct{}

func (c *noCache) Put(string, *tls.ClientSessionState) {}
func (c *noCache) Get(string) (*tls.ClientSessionState, bool) {
	return new(tls.ClientSessionState), false
}
