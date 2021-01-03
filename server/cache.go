package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ikedam/gtokenserver/internal/util"
	"github.com/ikedam/gtokenserver/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type cachedDefaultCredentials struct {
	Credentials      *google.Credentials
	ClientID         string
	ProjectID        string
	email            string
	numericProjectID int64
}

func newCachedDefaultCredentials(credentials *google.Credentials, projectID string) (*cachedDefaultCredentials, error) {
	clientID, err := util.GetEmailOfCredentials(credentials)
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		projectID = credentials.ProjectID
	}
	return &cachedDefaultCredentials{
		Credentials: credentials,
		ClientID:    clientID,
		ProjectID:   projectID,
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

type projectResponseHolder struct {
	ProjectNumber string `json:"projectNumber"`
}

func (c *cachedDefaultCredentials) GetNumericProjectID() (int64, error) {
	if c.ProjectID == "" {
		return 0, nil
	}
	if c.numericProjectID != 0 {
		return c.numericProjectID, nil
	}

	// https://cloud.google.com/resource-manager/reference/rest/v1/projects/get
	// It sounds really strange, but you need to enable API for service accounts.
	// It always works for authorized users.
	client := oauth2.NewClient(
		context.Background(),
		c.Credentials.TokenSource,
	)
	rsp, err := client.Get(fmt.Sprintf(
		"https://cloudresourcemanager.googleapis.com/v1/projects/%v",
		url.PathEscape(c.ProjectID),
	))
	if err != nil {
		return 0, fmt.Errorf("Failed to resolve numeric project number: %w", err)
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return 0, fmt.Errorf("Failed to resolve numeric project number: %w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		log.WithField("status", rsp.StatusCode).
			WithField("body", string(body)).
			Debugf("Unexpected response from project endpoint")
		return 0, fmt.Errorf("Unexpected response from project endpoint: %v", rsp.StatusCode)
	}

	var projectResponse projectResponseHolder
	if err := json.Unmarshal(body, &projectResponse); err != nil {
		log.WithField("body", string(body)).
			Debugf("Unexpected response from project endpoint")
		return 0, fmt.Errorf("Unexpected response from project endpoint: %v", rsp.StatusCode)
	}

	numericProjectID, err := strconv.ParseInt(projectResponse.ProjectNumber, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Unexpected response from project endpoint: %w", err)
	}

	c.numericProjectID = numericProjectID
	return numericProjectID, nil
}

func (c *cachedDefaultCredentials) Token() (*oauth2.Token, error) {
	return c.Credentials.TokenSource.Token()
}
