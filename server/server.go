// Package server provides a dummy metadata server providing tokens to access Google Cloud Platform
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ikedam/gtokenserver/internal/util"
	"github.com/ikedam/gtokenserver/log"

	"golang.org/x/oauth2/google"
)

// Config is a configuration to the server to launch
type Config struct {
	Host   string
	Port   int
	Scopes []string
}

type credentialsCache struct {
	clientID string
	email    string
}

func (c *credentialsCache) setClientID(clientID string) {
	if c.clientID == clientID {
		return
	}
	*c = credentialsCache{
		clientID: clientID,
	}
}

// Server is an instance of gtokenserver
type Server struct {
	config Config
	cache  credentialsCache
}

// NewServer creates a Server
func NewServer(config *Config) *Server {
	return &Server{
		config: *config,
	}
}

// Serve launches an instance of gtokenserver
func (s *Server) Serve() error {
	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(s.notFound)

	computeMetadataV1 := r.PathPrefix("/computeMetadata/v1").Subrouter()
	project := computeMetadataV1.PathPrefix("/project").Subrouter()
	project.HandleFunc("/numeric-project-id", s.handleProjectNumericProjectID)

	serviceAccounts := computeMetadataV1.PathPrefix("/instance/service-accounts").Subrouter()
	serviceAccounts.HandleFunc("/", s.handleServiceAccounts)

	serviceAccount := serviceAccounts.PathPrefix("/{account}").Subrouter()
	serviceAccount.HandleFunc("/", s.handleServiceAccount)
	serviceAccount.HandleFunc("/email", s.handleServiceAccountEmail)
	serviceAccount.HandleFunc("/token", s.handleServiceAccountToken)
	serviceAccount.HandleFunc("/identity", s.handleServiceAccountIdentity)

	srv := &http.Server{
		Handler: util.InstallHTTPLogger(checkMetadataFlavor(r)),
		Addr:    fmt.Sprintf("%v:%v", s.config.Host, s.config.Port),
	}

	return srv.ListenAndServe()
}

func checkMetadataFlavor(handler http.Handler) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			log.WithField("method", r.Method).
				WithField("path", r.URL.Path).
				Debug("Accessed without Metadata-Flavor: Google")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler.ServeHTTP(w, r)
	})
	return m
}

func (s *Server) getDefaultCredentials(scopes ...string) *google.Credentials {
	cred, err := google.FindDefaultCredentials(context.Background(), scopes...)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve default credentials")
		return nil
	}
	return cred
}

func (s *Server) handleProjectNumericProjectID(w http.ResponseWriter, r *http.Request) {
	// TODO
	s.writeTextResponse(w, "0")
}

func (s *Server) getEmail(cred *google.Credentials) (string, error) {
	clientID, err := util.GetIDOfCredentials(cred)
	if err != nil {
		return "", err
	}
	if s.cache.clientID == clientID && s.cache.email != "" {
		return s.cache.email, nil
	}
	email, err := util.GetEmailOfCredentials(cred)
	if err != nil {
		return "", err
	}
	s.cache.setClientID(clientID)
	s.cache.email = email
	return email, nil
}

func (s *Server) handleServiceAccounts(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := s.getEmail(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.writeTextResponse(w, fmt.Sprintf("default/\n%s\n", email))
}

type serviceAccountRecursiveResponse struct {
	Scopes  []string `json:"scopes"`
	Email   string   `json:"email"`
	Aliases []string `json:"aliases"`
}

func (s *Server) handleServiceAccount(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := s.getEmail(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	if vars["account"] != "default" && vars["account"] != email {
		log.WithField("method", r.Method).
			WithField("path", r.RequestURI).
			Warning("Access to an unknown account")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.URL.Query().Get("recursive") != "true" {
		s.writeTextResponse(w, "email/\nscopes/\ntoken\n")
		return
	}
	response := serviceAccountRecursiveResponse{
		Scopes:  s.config.Scopes,
		Email:   email,
		Aliases: []string{"default"},
	}
	s.writeJSONResponse(w, &response)
}

func (s *Server) handleServiceAccountEmail(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := s.getEmail(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	if vars["account"] != "default" && vars["account"] != email {
		log.WithField("method", r.Method).
			WithField("path", r.RequestURI).
			Warning("Access to an unknown account")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.writeTextResponse(w, email)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (s *Server) handleServiceAccountToken(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := s.getEmail(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	if vars["account"] != "default" && vars["account"] != email {
		log.WithField("method", r.Method).
			WithField("path", r.RequestURI).
			Warning("Access to an unknown account")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	scopes := s.config.Scopes
	if r.URL.Query().Get("scopes") != "" {
		scopes = strings.Split(r.URL.Query().Get("scopes"), ",")
	}
	cred = s.getDefaultCredentials(scopes...)
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	token, err := cred.TokenSource.Token()
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve token")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.writeJSONResponse(w, &tokenResponse{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresIn:   int(token.Expiry.Sub(time.Now()).Seconds()),
	})
}

func (s *Server) handleServiceAccountIdentity(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := s.getEmail(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	if vars["account"] != "default" && vars["account"] != email {
		log.WithField("method", r.Method).
			WithField("path", r.RequestURI).
			Warning("Access to an unknown account")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Warningf("/identity endpoint is not supported.")
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) writeTextResponse(w http.ResponseWriter, text string) {
	w.Header().Add("Metadata-Flavor", "Google")
	w.Header().Add("Content-Type", "application/text")
	w.Write([]byte(text))
}

func (s *Server) writeJSONResponse(w http.ResponseWriter, obj interface{}) {
	body, err := json.Marshal(obj)
	if err != nil {
		log.WithError(err).
			WithField("content", obj).
			Error("Failed to serialize response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Metadata-Flavor", "Google")
	w.Header().Add("Content-Type", "application/json")
	w.Write(body)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	log.WithField("method", r.Method).
		WithField("path", r.RequestURI).
		Warning(
			"Unimplemented path is accessed: " +
				"Please report in https://github.com/ikedam/gtokenserver/issues if your application doesn't work for this problem.",
		)
	w.WriteHeader(http.StatusNotFound)
}
