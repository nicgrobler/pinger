package main

import (
	"bytes"
	"context"
	"os"
	"testing"

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
