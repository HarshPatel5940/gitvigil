package github

import (
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v68/github"
)

type AppClient struct {
	appID      int64
	privateKey []byte
	transport  *ghinstallation.AppsTransport
	client     *github.Client
}

func NewAppClient(appID int64, privateKey []byte) (*AppClient, error) {
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("private key is required")
	}

	transport, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create apps transport: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport})

	return &AppClient{
		appID:      appID,
		privateKey: privateKey,
		transport:  transport,
		client:     client,
	}, nil
}

func (a *AppClient) GetInstallationClient(installationID int64) (*github.Client, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, a.appID, installationID, a.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %w", err)
	}

	return github.NewClient(&http.Client{Transport: transport}), nil
}

func (a *AppClient) AppClient() *github.Client {
	return a.client
}

func (a *AppClient) AppID() int64 {
	return a.appID
}
