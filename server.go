// Package gtokenserver provides a dummy metadata server providing tokens to access Google Cloud Platform
package gtokenserver

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ikedam/gtokenserver/log"
)

// ServerConfig is a configuration to the server to launch
type ServerConfig struct {
	Host string
	Port int
}

// Server is an instance of gtokenserver
type Server struct {
	config ServerConfig
}

// NewServer creates a Server
func NewServer(config *ServerConfig) *Server {
	return &Server{
		config: *config,
	}
}

// Serve launches an instance of gtokenserver
func (s *Server) Serve() error {
	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(s.notFound)

	srv := &http.Server{
		Handler: installHTTPLogger(r),
		Addr:    fmt.Sprintf("%v:%v", s.config.Host, s.config.Port),
	}

	return srv.ListenAndServe()
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	log.WithField("method", r.Method).
		WithField("path", r.URL.Path).
		Warningf(
			"Unimplemented path is accessed: " +
				"Please report in https://github.com/ikedam/gtokenserver/issues if your application doesn't work for this problem.",
		)
	w.WriteHeader(http.StatusNotFound)
}
