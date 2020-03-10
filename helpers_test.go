package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlterter(t *testing.T) {
	// wrong variable
	err := os.Setenv("STARTUP_RETRIES", "d")
	assert.Nil(t, err)

	// set logging into custom and testable location
	logOutput := &bytes.Buffer{}
	setLogging("localhost:1234", logOutput)
	ctx := signalContext()
	// start alerter
	errorChannel := make(chan error, 20)
	// set context so we can cancel the listner(s)
	ctx, cancel := context.WithCancel(context.Background())
	go alerter(ctx, errorChannel)
	cancel()

	c, err := getConfig()
	assert.NotNil(t, err)
	assert.Equal(t, &config{}, c)
	assert.Equal(t, "invalid value passed for STARTUP_RETRIES: strconv.Atoi: parsing \"d\": invalid syntax", err.Error())
	// confirm that we got an error logged
	assert.Contains(t, string(logOutput.Bytes()), "msg=\"logging to stderr & graylog2@'localhost:1234'\"")

}

func TestHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := &http.Request{Method: "POST"}
	c := &config{}

	handler(w, r, c)

	assert.Equal(t, "200 OK", w.Result().Status)

}

func TestServerStop(t *testing.T) {
	// set logging into custom and testable location
	logOutput := &bytes.Buffer{}
	setLogging("", logOutput)
	os.Setenv("STARTUP_RETRIES", "")
	ctx := signalContext()
	// start alerter
	errorChannel := make(chan error, 20)
	// set context so we can cancel the listner(s)
	ctx, cancel := context.WithCancel(context.Background())
	go alerter(ctx, errorChannel)

	cancel()

	c, err := getConfig()
	assert.Nil(t, err)

	// start server...
	startServer(ctx, c, errorChannel)

	// cancel context - as would happen when signalled
	cancel()

	// confirm that we got shutdown message in log
	messagesShouldBe := []string{
		"msg=\"http listener stopping\"",
		"msg=\"http listener stopped\"",
		"msg=\"server stopped\"",
	}
	for _, message := range messagesShouldBe {
		assert.Contains(t, string(logOutput.Bytes()), message)
	}

}

func TestGather(t *testing.T) {
	getPeersFunc := func(c *config) (peers, error) {
		return nil, errors.New("this is a bad error")
	}
	// set logging into custom and testable location
	logOutput := &bytes.Buffer{}
	config, err := getConfig()
	config.StartupDelay = 0
	assert.Nil(t, err)
	setLogging("", logOutput)
	os.Setenv("STARTUP_RETRIES", "")

	// start alerter
	errorChannel := make(chan error, 20)
	ctx, cancel := context.WithCancel(context.Background())
	go alerter(ctx, errorChannel)
	go gather(ctx, config, getPeersFunc, errorChannel)

	// need a delay to allow previous to actually DO something before killing them with cancel()
	time.Sleep(1 * time.Second)
	cancel()

	c, err := getConfig()
	assert.Nil(t, err)

	// start server...
	startServer(ctx, c, errorChannel)

	// cancel context - as would happen when signalled
	cancel()

	// confirm that we got shutdown message in log
	messagesShouldBe := []string{
		"msg=\"http listener stopping\"",
		"msg=\"http listener stopped\"",
		"msg=\"server stopped\"",
		"msg=\"this is a bad error\"",
	}
	for _, message := range messagesShouldBe {
		assert.Contains(t, string(logOutput.Bytes()), message)
	}

}
