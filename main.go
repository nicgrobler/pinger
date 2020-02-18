package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	gelf "github.com/Graylog2/go-gelf/gelf"
	log "github.com/sirupsen/logrus"
)

const (
	ENDPOINT     string = "/ping"
	ERROR_PREFIX        = "error:"
	OK_PREFIX           = "ok:"
)

type config struct {
	MyID                   int
	MaxID                  int
	BaseName               string
	MyName                 string
	Seperator              string
	ConnectionCloseTimeout time.Duration
	IdleConnectionTimeout  time.Duration
	CycleTime              time.Duration
	Port                   string
	URL                    string
}

type peers map[string]int

func getValidPeerList(c config) (peers, error) {
	p := make(peers)
	for i := 1; i <= c.MaxID; i++ {
		if i != c.MyID {
			p[c.BaseName+c.Seperator+strconv.Itoa(i)] = 0
		}
	}
	if len(p) == 0 {
		return p, errors.New("not enough peernames supplied")
	}
	return p, nil
}

// signalContext listens for os signals, and when received, calls cancel on returned context.
func signalContext() context.Context {

	// listen for any and all signals
	c := make(chan os.Signal, 1)
	signal.Notify(c)

	// set context so we can cancel the listner(s)
	ctx, cancel := context.WithCancel(context.Background())

	// prepare to cancel context on receipt of os signal
	go func() {
		oscall := <-c
		fmt.Printf("received signal: %+v\n", oscall)
		cancel()
	}()

	return ctx

}

func alerter(c context.Context, cfg config, errorChannel chan error) {
	// split logs to logger and gelf IF we were given a URL
	if cfg.URL != "" {
		graylogAddr := cfg.URL
		gelfWriter, err := gelf.NewWriter(graylogAddr)
		// If using TCP
		//gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
		if err != nil {
			log.Fatalf("gelf.NewWriter: %s", err)
		}
		// log to both stderr and graylog2
		log.SetOutput(io.MultiWriter(os.Stderr, gelfWriter))
		log.Printf("logging to stderr & graylog2@'%s'", graylogAddr)

	} else {
		log.Info("logging to stderr only - no graylog url supplied")
	}

	for {
		select {
		case err := <-errorChannel:
			// this can be sent to the destination URL
			message := err.Error()
			if message[0] == ERROR_PREFIX[0] {
				log.Error(message)
			} else {
				log.Info(message)
			}
		case <-c.Done():
			return
		}
	}
}

func addEnvToConfig(c *config) {

	basename := os.Getenv("BASENAME")
	if basename != "" {
		c.BaseName = basename
	} else {
		c.BaseName = "pinger"
	}

	seperator := os.Getenv("SEPERATOR")
	if seperator != "" {
		c.Seperator = seperator
	} else {
		c.BaseName = "_"
	}

	id := os.Getenv("ID")
	if id != "" {
		s, err := strconv.Atoi(id)
		if err != nil {
			log.Fatalf("invalid value passed for ID: %s", err.Error())
		}
		c.MyID = s
	} else {
		c.MyID = 1
	}

	maxid := os.Getenv("MAXID")
	if maxid != "" {
		s, err := strconv.Atoi(maxid)
		if err != nil {
			log.Fatalf("invalid value passed for MAXID: %s", err.Error())
		}
		c.MaxID = s
	} else {
		c.MaxID = 10
	}

	timeoutString := os.Getenv("CONNECTION_TIMEOUT_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			log.Fatalf("invalid value passed for CONNECTION_CLOSE_TIME_LIMIT: %s", err.Error())
		}
		c.ConnectionCloseTimeout = time.Duration(s) * time.Second
	} else {
		c.ConnectionCloseTimeout = 1
	}

	timeoutString = os.Getenv("IDLE_CONNECTION_TIMEOUT_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			log.Fatalf("invalid value passed for CONNECTION_CLOSE_TIME_LIMIT: %s", err.Error())
		}
		c.IdleConnectionTimeout = time.Duration(s) * time.Second
	} else {
		c.IdleConnectionTimeout = 1
	}

	timeoutString = os.Getenv("CYCLE_TIME_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			log.Fatalf("invalid value passed for CONNECTION_CLOSE_TIME_LIMIT: %s", err.Error())
		}
		c.CycleTime = time.Duration(s) * time.Second
	} else {
		c.CycleTime = 10
	}

	portString := os.Getenv("PORT")
	if portString != "" {
		c.Port = portString
	} else {
		c.Port = "8123"
	}

	c.URL = os.Getenv("GELF_URL")

	c.MyName = buildName(c.Seperator, c.BaseName, c.MyID)
}

func buildName(seperator, basename string, id int) string {
	return basename + seperator + strconv.Itoa(id)
}

func main() {

	c := config{}

	// add environment variables to config
	addEnvToConfig(&c)

	peerList, err := getValidPeerList(c)
	if err != nil {
		log.Fatalf("stopping due to error: %s", err.Error())
		return
	}

	ctx := signalContext()

	errorChannel := make(chan error, c.MaxID)
	go alerter(ctx, c, errorChannel)

	// dispatch all and wait...
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func(w *sync.WaitGroup) {
		startServer(ctx, c, peerList, errorChannel)
		wg.Done()
	}(&wg)
	go func(w *sync.WaitGroup) {
		startClient(ctx, c, peerList, errorChannel)
		wg.Done()
	}(&wg)
	// now block and wait
	wg.Wait()
}
