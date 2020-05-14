package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type HTTPClient struct {
	c            *http.Client
	errorChannel chan error
	port         string
}

func (client *HTTPClient) doConnect(ip string, wg *sync.WaitGroup) {
	defer wg.Done()

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
	url := "http://" + host + ":" + client.port + ENDPOINT
	response, err := client.c.Get(url)
	// we don't use this - but must close it
	response.Body.Close()
	if err != nil {
		client.errorChannel <- errors.New(ERROR_PREFIX + err.Error())
		return
	}

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
			Timeout: time.Duration(c.ConnectionCloseTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(c.ConnectionCloseTimeout) * time.Second,
	}
	netClient := &http.Client{
		Timeout:   time.Duration(c.ConnectionCloseTimeout) * time.Second,
		Transport: netTransport,
	}

	return HTTPClient{c: netClient, errorChannel: errorChannel, port: c.Port}

}

func (client *HTTPClient) gather(c config, getPeerFunc func(config) (peers, error), wg *sync.WaitGroup) {

	p, err := getPeerFunc(c)
	if err != nil {
		client.errorChannel <- err
		log.Println("client stopping due to error")
		return
	}

	wg.Add(len(p))
	// despatch the http clients here
	for _, peer := range p {
		go client.doConnect(peer, wg)
	}
	wg.Wait()

}

func startClient(ctx context.Context, c config, errorChannel chan error) {
	// start the gather with cycle time - this will block here
	cycle := time.NewTicker(time.Duration(c.CycleTime) * time.Second)
	log.Println("delayed startup wait...")
	time.Sleep(time.Duration(c.StartupDelay) * time.Second)
	log.Println("starting client...")
	client := getClient(errorChannel, c)
	wg := sync.WaitGroup{}
	for {
		// should we exit or not
		select {
		case <-cycle.C:
			client.gather(c, getValidPeerList, &wg)
		case <-ctx.Done():
			log.Println("client stopping")
			return
		}
	}
}
