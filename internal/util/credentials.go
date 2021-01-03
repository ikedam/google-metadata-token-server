package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ikedam/gtokenserver/log"

	"golang.org/x/oauth2/google"
)

type credentialsJSON struct {
	ClientID    string `json:"client_id,omitempty"`
	Type        string `json:"type,omitempty"`
	ClientEmail string `json:"client_email,omitempty"`
}

const (
	typeAuthorizedUser = "authorized_user"
	typeServiceAccount = "service_account"

	userInfoEndpoint = "https://www.googleapis.com/oauth2/v1/userinfo"
)

// GetIDOfCredentials returns ID of the credentials
func GetIDOfCredentials(cred *google.Credentials) (string, error) {
	var c credentialsJSON
	if err := json.Unmarshal(cred.JSON, &c); err != nil {
		return "", fmt.Errorf("Failed to parse credentials JSON: %w", err)
	}
	return c.ClientID, nil
}

// GetEmailOfCredentials returns Email of the credentials
func GetEmailOfCredentials(cred *google.Credentials) (string, error) {
	var c credentialsJSON
	if err := json.Unmarshal(cred.JSON, &c); err != nil {
		return "", fmt.Errorf("Failed to parse credentials JSON: %w", err)
	}

	switch c.Type {
	case typeAuthorizedUser:
		return getEmailOfAuthorizedUser(cred)
	case typeServiceAccount:
		return c.ClientEmail, nil
	}

	return "", fmt.Errorf("Unexpected type: %v", c.Type)
}

type userInfoResponse struct {
	Email string
}

func getEmailOfAuthorizedUser(cred *google.Credentials) (string, error) {
	// "authorized_user" credentials don't store email address.
	// Then query it to the userinfo endpoint.
	// This doesn't work for "service_account" credentials.
	token, err := cred.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("Failed to get token to resolve email: %w", err)
	}
	userInfoURL, err := url.Parse(userInfoEndpoint)
	if err != nil {
		return "", fmt.Errorf("Failed to resolve the userinfo endpoint: %w", err)
	}
	q := userInfoURL.Query()
	q.Add("access_token", token.AccessToken)
	userInfoURL.RawQuery = q.Encode()
	c := http.Client{}
	rsp, err := c.Get(userInfoURL.String())
	if err != nil {
		return "", fmt.Errorf("Failed to access the userinfo endpoint: %w", err)
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read response from the userinfo endpoint: %w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		log.WithField("status", rsp.StatusCode).
			WithField("body", string(body)).
			Debugf("Unexpected response from userinfo endpoint")
		return "", fmt.Errorf("Unexpected response from the userinfo endpoint: %v", rsp.StatusCode)
	}
	var userInfo userInfoResponse
	if err := json.Unmarshal(body, &userInfo); err != nil {
		log.WithField("body", string(body)).
			Debugf("Unexpected response from userinfo endpoint")
		return "", fmt.Errorf("Failed to parse response from the userinfo endpoint: %err", err)
	}
	return userInfo.Email, nil
}
