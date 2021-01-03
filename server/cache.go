package server

import (
	"github.com/ikedam/gtokenserver/internal/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type cachedDefaultCredentials struct {
	Credentials *google.Credentials
	ClientID    string
	email       string
}

func newCachedDefaultCredentials(credentials *google.Credentials) (*cachedDefaultCredentials, error) {
	clientID, err := util.GetEmailOfCredentials(credentials)
	if err != nil {
		return nil, err
	}
	return &cachedDefaultCredentials{
		Credentials: credentials,
		ClientID:    clientID,
	}, nil
}

func (c *cachedDefaultCredentials) GetEmail() (string, error) {
	if c.email != "" {
		return c.email, nil
	}
	email, err := util.GetEmailOfCredentials(c.Credentials)
	if err != nil {
		return "", err
	}
	c.email = email
	return email, nil
}

func (c *cachedDefaultCredentials) Token() (*oauth2.Token, error) {
	return c.Credentials.TokenSource.Token()
}
