package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type status struct {
	Status string `json:"status"`
}

func handler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET", "POST":
		s := status{Status: "ok"}
		if err := json.NewEncoder(w).Encode(s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "unsupported method call\n")
	}
}

// HttpServer is the basic abstraction used for handling http requests
type HttpServer struct {
	slug         string
	address      string
	Done         chan error
	router       *http.ServeMux
	listener     *http.Server
	errorChannel chan error
}

// NewHTTPServer takes a valid address - can be of form IP:Port, or :Port - and returns a server
func NewHTTPServer(description, address string, errorChannel chan error, c *config) *HttpServer {
	s := &HttpServer{slug: description, address: address, Done: make(chan error), router: http.NewServeMux(), errorChannel: errorChannel}
	s.setListener(&http.Server{Addr: address, Handler: s.router, IdleTimeout: time.Duration(c.IdleConnectionTimeout) * time.Second})
	return s
}

func (s *HttpServer) setListener(l *http.Server) {
	s.listener = l
}

// RegisterHandler allows caller to set routing and handler functions as needed
func (s *HttpServer) RegisterHandler(path string, handlerfn func(http.ResponseWriter, *http.Request)) {
	s.router.HandleFunc(path, handlerfn)
}

// StartListener starts the server's listener with a context, allowing for later graceful shutdown.
// the supplied timeout is the amount of time that is allowed before the server forcefully
// closes any remaining conections. Once done close Done channel
// note: this is a blocking call
func (s *HttpServer) StartListener(ctx context.Context, timeout time.Duration) {

	go func() {
		if err := s.listener.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.errorChannel <- err
		}
	}()

	log.Println(s.slug + " on: " + s.address)

	<-ctx.Done()

	log.Println(s.slug + " stopping")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), timeout)
	defer func() {
		cancel()
	}()

	if err := s.listener.Shutdown(ctxShutDown); err != nil {
		log.Println(s.slug+" graceful shutdown failed:%+s", err)
		if e := s.listener.Close(); e != nil {
			log.Fatalf(s.slug+" forced shutdown failed:%+s", err)
		} else {
			log.Println("forced shutdown ok")
		}
	}

	log.Println(s.slug + " stopped")

	// let parent know that we are done
	close(s.Done)
}

func startServer(ctx context.Context, c *config, errorChannel chan error) {

	// initialise http listener
	httpServer := NewHTTPServer("http listener", "0.0.0.0:"+c.Port, errorChannel, c)
	httpServer.RegisterHandler(ENDPOINT, func(w http.ResponseWriter, r *http.Request) { handler(w, r) })

	// run http server's listener
	go httpServer.StartListener(ctx, time.Duration(c.ConnectionCloseTimeout))

	// wait for all to complete
	<-httpServer.Done

}
