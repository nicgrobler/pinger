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

func doConnect(ip string, errorChannel chan error, c *config, wg *sync.WaitGroup) {
	defer wg.Done()
	client := getClient(errorChannel, c)
	// as testing name resolution as well as connectivity, perform addr resolution first
	name, err := net.LookupAddr(ip)
	if err != nil {
		client.errorChannel <- errors.New(ERROR_PREFIX + err.Error())
		return
	}
	// chop off the last '.' from the address
	host := name[0]
	if host[len(host)-1] == byte('.') {
		host = host[0 : len(host)-1]
	}
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

func getClient(errorChannel chan error, c *config) HTTPClient {
	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(c.ConnectionCloseTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(c.ConnectionCloseTimeout) * time.Second,
	}
	netClient := &http.Client{
		Timeout:   time.Duration(c.ConnectionCloseTimeout) * time.Second,
		Transport: netTransport,
	}

	return HTTPClient{c: netClient, errorChannel: errorChannel}

}

func gather(con context.Context, c *config, getPeerFunc func(*config) (peers, error), errorChannel chan error) {

	wg := sync.WaitGroup{}
	cycle := time.NewTicker(time.Duration(c.CycleTime) * time.Second)
	log.Info("starting clients...")

	for {
		p, err := getPeerFunc(c)
		if err != nil {
			errorChannel <- err
			log.Error("client stopping due to error")
			return
		}
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

func startClient(ctx context.Context, c *config, errorChannel chan error) {
	// start the gather with cycle time - this will block here
	gather(ctx, c, getValidPeerList, errorChannel)
	log.Info("client stopped")
}
