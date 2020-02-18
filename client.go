package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type HTTPClient struct {
	c            *http.Client
	errorChannel chan error
}

func doConnect(host string, errorChannel chan error, c config, wg *sync.WaitGroup) {
	defer wg.Done()
	client := getClient(errorChannel, c)
	url := "http://" + host + ":" + c.Port + ENDPOINT
	response, err := client.c.Get(url)
	if err != nil {
		client.errorChannel <- errors.New(ERROR_PREFIX + err.Error())
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		client.errorChannel <- errors.New(ERROR_PREFIX + "server returned failure status: " + strconv.Itoa(response.StatusCode))
		return
	}

	// if we get here, we have a good response - so create a 0 error
	client.errorChannel <- errors.New(OK_PREFIX + url + " - returned ok")
}

func getClient(errorChannel chan error, c config) HTTPClient {
	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: c.ConnectionCloseTimeout,
		}).Dial,
		TLSHandshakeTimeout: c.ConnectionCloseTimeout,
	}
	netClient := &http.Client{
		Timeout:   c.ConnectionCloseTimeout,
		Transport: netTransport,
	}

	return HTTPClient{c: netClient, errorChannel: errorChannel}

}

func gather(con context.Context, c config, p peers, errorChannel chan error) {

	wg := sync.WaitGroup{}
	cycle := time.NewTicker(c.CycleTime)
	log.Info("starting clients...")

	for {
		// should we exit or not
		select {
		case <-cycle.C:
			wg.Add(len(p))

			// do the work...
			for peer := range p {
				go doConnect(peer, errorChannel, c, &wg)
			}
			wg.Wait()
		case <-con.Done():
			log.Info("client stopping")
			return
		}
	}
}

func startClient(ctx context.Context, c config, p peers, errorChannel chan error) {
	// start the gather with cycle time - this will block here
	gather(ctx, c, p, errorChannel)
	log.Info("client stopped")
}
