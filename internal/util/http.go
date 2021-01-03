package gtokenserver

import (
	"net/http"

	"github.com/ikedam/gtokenserver/log"
)

func installHTTPLogger(handler http.Handler) *http.ServeMux {
	logMux := http.NewServeMux()
	logMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rspWrapper := newResponseSniffer(w)
		handler.ServeHTTP(rspWrapper, r)
		log.Infof("%+v %+v %+v size=%v", r.Method, r.RequestURI, rspWrapper.Code(), rspWrapper.BodySize())
	})
	return logMux
}

// responseSniffer wraps http.ResponseWriter
type responseSniffer struct {
	writer   http.ResponseWriter
	code     int
	bodySize int
}

// NewResponseSniffer creates a new ResponseSniffer
func newResponseSniffer(writer http.ResponseWriter) *responseSniffer {
	return &responseSniffer{
		writer: writer,
	}
}

// Code returns status code
func (s *responseSniffer) Code() int {
	return s.code
}

// BodySize returns response body size
func (s *responseSniffer) BodySize() int {
	return s.bodySize
}

// Header returns Header object to write headers to.
func (s *responseSniffer) Header() http.Header {
	return s.writer.Header()
}

// Write writes response body.
func (s *responseSniffer) Write(body []byte) (int, error) {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	s.bodySize += len(body)
	return s.writer.Write(body)
}

// WriteHeader writes status code.
func (s *responseSniffer) WriteHeader(statusCode int) {
	s.code = statusCode
	s.writer.WriteHeader(statusCode)
}
