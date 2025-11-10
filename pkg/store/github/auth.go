// SPDX-FileCopyrightText: 2022 - 2025 NRK
//
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v76/github"
	"golang.org/x/oauth2"
)

type githubTokenSource struct {
	PrivatePem    []byte
	ApplicationID string
	// If installationID is not provided, the first one is chosen
	InstallationID int64
	// Leave Repos as empty list to get token for all repos
	Repos []string

	privateKey crypto.PrivateKey
	token      *oauth2.Token
	lock       sync.Mutex
}

// newGithubClient returns an authenticated github client lasting 1 hour
// leave repos as empty string if you need a token for all repos
func (gts *githubTokenSource) Token() (*oauth2.Token, error) {
	gts.lock.Lock()
	defer gts.lock.Unlock()

	var err error
	if gts.token != nil && time.Until(gts.token.Expiry) > 2*time.Minute {
		return gts.token, nil
	}

	if gts.privateKey == nil {
		block, _ := pem.Decode([]byte(gts.PrivatePem))
		gts.privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	unsignedToken := jwt.New(jwt.SigningMethodRS256)
	claims := unsignedToken.Claims.(jwt.MapClaims)
	claims["iat"] = time.Now().Add(-1 * time.Minute).Unix()
	claims["exp"] = time.Now().Add(4 * time.Minute).Unix()
	claims["iss"] = gts.ApplicationID

	signedToken, err := unsignedToken.SignedString(gts.privateKey)
	if err != nil {
		return nil, err
	}

	tmpHttpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: signedToken}))
	tmpClient := github.NewClient(tmpHttpClient)

	if gts.InstallationID == 0 {
		installations, _, err := tmpClient.Apps.ListInstallations(context.Background(), &github.ListOptions{})
		if err != nil {
			return nil, err
		}
		gts.InstallationID = *installations[0].ID
	}

	installationToken, _, err := tmpClient.Apps.CreateInstallationToken(context.Background(), gts.InstallationID, &github.InstallationTokenOptions{
		Repositories: gts.Repos,
	})
	if err != nil {
		return nil, err
	}

	gts.token = &oauth2.Token{
		AccessToken: installationToken.GetToken(),
		Expiry:      installationToken.GetExpiresAt().Time,
		TokenType:   "Bearer",
	}

	return gts.token, nil
}

// NewGithubClient creates a github.Client, with an automatically renew token
// privatePem is the bytestring of the privatekey you download from the github app installation
// set installationID to 0 if you only have one installation
// leave repos empty if you want a token for all repos
func newGithubClient(privatePem []byte, applicationID string, installationID int64, repos ...string) (client *github.Client) {
	httpClient := oauth2.NewClient(context.Background(), &githubTokenSource{
		PrivatePem:     privatePem,
		ApplicationID:  applicationID,
		Repos:          repos,
		InstallationID: installationID,
	})
	return github.NewClient(httpClient)
}
