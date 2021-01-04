// Package server provides a dummy metadata server providing tokens to access Google Cloud Platform
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ikedam/gtokenserver/internal/util"
	"github.com/ikedam/gtokenserver/log"

	"golang.org/x/oauth2/google"
)

// Config is a configuration to the server to launch
type Config struct {
	Host                         string
	Port                         int
	Scopes                       []string
	Project                      string
	CloudSDKConfig               string `mapstructure:"cloudsdk-config"`
	GoogleApplicationCredentials string `mapstructure:"google-application-credentials"`
}

// Server is an instance of gtokenserver
type Server struct {
	config                           Config
	cache                            *cachedDefaultCredentials
	warnGoogleApplicationCredentials bool
	warnCoudSDKConfig                bool
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
	project.HandleFunc("/project-id", s.handleProjectProjectID)
	project.HandleFunc("/numeric-project-id", s.handleProjectNumericProjectID)

	serviceAccounts := computeMetadataV1.PathPrefix("/instance/service-accounts").Subrouter()
	serviceAccounts.HandleFunc("/", s.handleServiceAccounts)

	serviceAccount := serviceAccounts.PathPrefix("/{account}").Subrouter()
	serviceAccount.Use(s.serviceAccountMiddleware)
	serviceAccount.HandleFunc("/", s.handleServiceAccount)
	serviceAccount.HandleFunc("/email", s.handleServiceAccountEmail)
	serviceAccount.HandleFunc("/token", s.handleServiceAccountToken)
	serviceAccount.HandleFunc("/identity", s.handleServiceAccountIdentity)

	hostport := fmt.Sprintf("%v:%v", s.config.Host, s.config.Port)
	addr, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("Failed to listen %v: %w", hostport, err)
	}
	defer addr.Close()
	srv := &http.Server{
		Handler: util.InstallHTTPLogger(checkMetadataFlavor(r)),
	}

	log.Infof("Listening %v...", addr.Addr().String())

	return srv.Serve(addr)
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

func (s *Server) credentialsFromFile(ctx context.Context, file string, scopes ...string) (*google.Credentials, error) {
	body, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to read from %v: %w", file, err)
	}
	return google.CredentialsFromJSON(ctx, body, scopes...)
}

func (s *Server) findCredentials(scopes ...string) (*google.Credentials, error) {
	ctx := context.Background()
	if s.config.GoogleApplicationCredentials != "" {
		file, err := os.Stat(s.config.GoogleApplicationCredentials)
		if !os.IsNotExist(err) && !file.IsDir() {
			cred, err := s.credentialsFromFile(ctx, s.config.GoogleApplicationCredentials, scopes...)
			if err == nil { // Be careful: not != but ==
				s.warnGoogleApplicationCredentials = false
				return cred, nil
			}
			if !s.warnGoogleApplicationCredentials {
				s.warnGoogleApplicationCredentials = true
				log.WithError(err).
					WithField("file", s.config.GoogleApplicationCredentials).
					Warning("Failed to load specified credentials file: ignored.")
			}
		} else {
			if !s.warnGoogleApplicationCredentials {
				s.warnGoogleApplicationCredentials = true
				log.WithField("file", s.config.GoogleApplicationCredentials).
					Warning("Failed to stat specified credentials file: ignored.")
			}
		}
	}
	cloudSDKConfig := s.config.CloudSDKConfig
	if cloudSDKConfig == "" {
		// CLOUDSDK_CONFIG doesn't supported in golang oauth2 library.
		// See: https://github.com/googleapis/google-cloud-go/issues/288
		cloudSDKConfig = os.Getenv("CLOUDSDK_CONFIG")
	}
	if cloudSDKConfig != "" {
		applicationConfig := filepath.Join(cloudSDKConfig, "application_default_credentials.json")
		file, err := os.Stat(applicationConfig)
		if !os.IsNotExist(err) && !file.IsDir() {
			cred, err := s.credentialsFromFile(ctx, applicationConfig, scopes...)
			if err == nil { // Be careful: not != but ==
				s.warnCoudSDKConfig = false
				return cred, nil
			}
			if !s.warnCoudSDKConfig {
				s.warnCoudSDKConfig = true
				log.WithError(err).
					WithField("directory", applicationConfig).
					Warning("Failed to load credentials from specified cloud-sdk configuration directory: ignored.")
			}
		}
	}
	return google.FindDefaultCredentials(ctx, scopes...)
}

func (s *Server) getCredentials(scopes ...string) *cachedDefaultCredentials {
	actualScopes := scopes
	if scopes == nil {
		actualScopes = s.config.Scopes
	}
	cred, err := s.findCredentials(actualScopes...)
	if err != nil {
		log.WithError(err).
			Error(
				"Could not retrieve default credentials\n" +
					"You may haven't set up credentials. You can set up your credentials in one of those ways:\n" +
					"\n" +
					"  * Run `gcloud auth application-default login`. Share /root/.config/gcloud with volume mounts in docker containers.\n" +
					"  * Put the service account key file (a json file), and specify the path with GOOGLE_APPLICATION_CREDENTIALS environment variable.\n" +
					"\n\n",
			)
		return nil
	}
	newCache, err := newCachedDefaultCredentials(cred, s.config.Project)
	if err != nil {
		log.WithError(err).
			Error("Could not resolve default credentials")
		return nil
	}
	if scopes != nil {
		// Don't cache if scopes are explicitly specified.
		return newCache
	}
	cached := lastCachedDefaultCredentials
	if cached != nil && cached.ClientID == newCache.ClientID {
		return cached
	}
	lastCachedDefaultCredentials = newCache
	email, err := lastCachedDefaultCredentials.GetEmail()
	if err == nil { // Be careful: not err != nil, but err == nil
		log.Infof("New credentials: %v", email)
	} else {
		log.Infof("New credentials: client_id=%v", newCache.ClientID)
	}
	return newCache
}

func (s *Server) handleProjectProjectID(w http.ResponseWriter, r *http.Request) {
	cred := s.getCredentials()
	if cred == nil {
		s.writeTextResponse(w, "")
		return
	}
	s.writeTextResponse(w, cred.ProjectID)
}

func (s *Server) handleProjectNumericProjectID(w http.ResponseWriter, r *http.Request) {
	cred := s.getCredentials()
	if cred == nil {
		s.writeTextResponse(w, "0")
		return
	}
	numericProjectID, err := cred.GetNumericProjectID()
	if err != nil {
		log.WithError(err).Error("Failed to resolve numeric project id")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.writeTextResponse(w, fmt.Sprintf("%v", numericProjectID))
}

func (s *Server) handleServiceAccounts(w http.ResponseWriter, r *http.Request) {
	cred := s.getCredentials()
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
		cred := s.getCredentials()
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
		cred = s.getCredentials(strings.Split(r.URL.Query().Get("scopes"), ",")...)
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
