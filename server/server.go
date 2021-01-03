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

// Server is an instance of gtokenserver
type Server struct {
	config Config
	cache  *cachedDefaultCredentials
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
	serviceAccount.Use(s.serviceAccountMiddleware)
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

var lastCachedDefaultCredentials *cachedDefaultCredentials

func (s *Server) getDefaultCredentials(scopes ...string) *cachedDefaultCredentials {
	actualScopes := scopes
	if scopes == nil {
		actualScopes = s.config.Scopes
	}
	cred, err := google.FindDefaultCredentials(context.Background(), actualScopes...)
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve default credentials")
		return nil
	}
	newCache, err := newCachedDefaultCredentials(cred)
	if err != nil {
		log.WithError(err).
			Error("Could not resolve default credentials")
		return nil
	}
	if actualScopes != nil {
		// Don't cache if scopes are explicitly specified.
		return newCache
	}
	cached := lastCachedDefaultCredentials
	if cached != nil && cached.ClientID == newCache.ClientID {
		return cached
	}
	lastCachedDefaultCredentials = newCache
	return newCache
}

func (s *Server) handleProjectNumericProjectID(w http.ResponseWriter, r *http.Request) {
	// TODO
	s.writeTextResponse(w, "0")
}

func (s *Server) handleServiceAccounts(w http.ResponseWriter, r *http.Request) {
	cred := s.getDefaultCredentials()
	if cred == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	email, err := cred.GetEmail()
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.writeTextResponse(w, fmt.Sprintf("default/\n%s\n", email))
}

var credentialsKey = "credentials"

func (s *Server) serviceAccountMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cred := s.getDefaultCredentials()
		if cred == nil {
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), &credentialsKey, cred))

		vars := mux.Vars(r)
		if vars["account"] == "default" {
			next.ServeHTTP(w, r)
			return
		}

		// verify email
		email, err := cred.GetEmail()
		if err != nil {
			log.WithError(err).
				Error("Could not retrieve email of the credential")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if vars["account"] != email {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getCredentialsFromContext(ctx context.Context) *cachedDefaultCredentials {
	return ctx.Value(&credentialsKey).(*cachedDefaultCredentials)
}

type serviceAccountRecursiveResponse struct {
	Scopes  []string `json:"scopes"`
	Email   string   `json:"email"`
	Aliases []string `json:"aliases"`
}

func (s *Server) handleServiceAccount(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("recursive") != "true" {
		s.writeTextResponse(w, "email/\nscopes/\ntoken\n")
		return
	}
	cred := s.getCredentialsFromContext(r.Context())
	email, err := cred.GetEmail()
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
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
	cred := s.getCredentialsFromContext(r.Context())
	email, err := cred.GetEmail()
	if err != nil {
		log.WithError(err).
			Error("Could not retrieve email of the credential")
		w.WriteHeader(http.StatusInternalServerError)
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
	cred := s.getCredentialsFromContext(r.Context())
	scopes := r.URL.Query().Get("scopes")
	if scopes != "" {
		cred = s.getDefaultCredentials(strings.Split(r.URL.Query().Get("scopes"), ",")...)
		if cred == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	token, err := cred.Token()
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
