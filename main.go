package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
	ConnectionCloseTimeout int
	IdleConnectionTimeout  int
	StartupRetries         int
	StartupRetryDelay      int
	StartupDelay           int
	CycleTime              int
	Port                   string
}

type peers map[string]int

func getValidPeerList(c *config) (peers, error) {
	p := make(peers)
	// get my own hostname
	myName, err := getHostName()
	if err != nil {
		return p, errors.New("failed to get own host name: " + err.Error())
	}

	// get my own ips as a map
	ips, err := getMyIPs(myName)
	if err != nil {
		return p, errors.New("failed to get own ips: " + err.Error())
	}

	// get the list of IPs for all tasks sitting behind our service - ignore error here as can fail at startup / scale operations
	others, err := net.LookupIP("tasks.testpinger_pinger.")

	if err != nil {
		// retry several times if needed
		for i := 0; i < c.StartupRetries; i++ {
			others, err = net.LookupIP("tasks.testpinger_pinger.")
			if err == nil {
				break
			}
			time.Sleep(time.Duration(c.StartupRetryDelay) * time.Second)
		}
	}

	if err != nil {
		return p, errors.New("failed to get tasks: " + err.Error())
	}

	// build list of task ips, that do not include our own
	for _, ip := range others {
		ipString := ip.String()
		_, found := ips[ipString]
		if !found {
			p[ipString] = 1
		}
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

func setLogging(graylogAddr string, logOutPut io.Writer) {
	// split logs to logger and gelf IF we were given a URL
	if graylogAddr != "" {
		gelfWriter, err := gelf.NewWriter(graylogAddr)
		// If using TCP
		//gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
		if err != nil {
			log.Fatalf("gelf.NewWriter: %s", err)
		}
		// log to both io.writer (stderr) and graylog2
		log.SetOutput(io.MultiWriter(logOutPut, gelfWriter))
		log.Printf("logging to stderr & graylog2@'%s'", graylogAddr)

	} else {
		log.SetOutput(logOutPut)
		log.Info("no graylog url supplied, will only log to default")
	}
}

func alerter(c context.Context, errorChannel chan error) {
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

func getHostName() (string, error) {
	return os.Hostname()
}

func getMyIPs(name string) (map[string]string, error) {
	ips := make(map[string]string)

	myIPs, err := net.LookupIP(name)
	if err != nil {
		return ips, err
	}
	for i := range myIPs {
		x := myIPs[i].String()
		ips[x] = x
	}
	return ips, nil
}

func getConfig() (*config, error) {

	c := &config{}

	timeoutString := os.Getenv("STARTUP_RETRIES")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for STARTUP_RETRIES: " + err.Error())
		}
		c.StartupRetries = s
	} else {
		c.StartupRetries = 1
	}

	timeoutString = os.Getenv("STARTUP_DELAY_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for STARTUP_DELAY_SECONDS: " + err.Error())
		}
		c.StartupDelay = s
	} else {
		c.StartupDelay = 1
	}

	timeoutString = os.Getenv("STARTUP_RETRIES_DELAY_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for STARTUP_RETRIES_DELAY_SECONDS: " + err.Error())
		}
		c.StartupRetryDelay = s
	} else {
		c.StartupRetryDelay = 1
	}

	timeoutString = os.Getenv("CONNECTION_TIMEOUT_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for CONNECTION_TIMEOUT_SECONDS: " + err.Error())
		}
		c.ConnectionCloseTimeout = s
	} else {
		c.ConnectionCloseTimeout = 1
	}

	timeoutString = os.Getenv("IDLE_CONNECTION_TIMEOUT_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for IDLE_CONNECTION_TIMEOUT_SECONDS: " + err.Error())
		}
		c.IdleConnectionTimeout = s
	} else {
		c.IdleConnectionTimeout = 1
	}

	timeoutString = os.Getenv("CYCLE_TIME_SECONDS")
	if timeoutString != "" {
		s, err := strconv.Atoi(timeoutString)
		if err != nil {
			return c, errors.New("invalid value passed for CYCLE_TIME_SECONDS: " + err.Error())
		}
		c.CycleTime = s
	} else {
		c.CycleTime = 10
	}

	portString := os.Getenv("PORT")
	if portString != "" {
		c.Port = portString
	} else {
		c.Port = "8111"
	}

	return c, nil
}

func main() {

	// set logging
	gelfUrl := os.Getenv("GELF_URL")
	logOutput := os.Stderr
	setLogging(gelfUrl, logOutput)

	// get config
	c, err := getConfig()
	if err != nil {
		log.Fatalf("startup failed due to a config error: %s", err.Error())
	}

	ctx := signalContext()

	errorChannel := make(chan error, 20)
	go alerter(ctx, errorChannel)

	// dispatch all and wait...
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func(w *sync.WaitGroup) {
		startServer(ctx, c, errorChannel)
		wg.Done()
	}(&wg)
	go func(w *sync.WaitGroup) {
		startClient(ctx, c, errorChannel)
		wg.Done()
	}(&wg)
	// now block and wait
	wg.Wait()

}
