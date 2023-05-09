package google

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOauth2 struct {
	config oauth2.Config
	logger *zap.Logger
}

func NewGoogleOauth2(clientId, clientSecret string, logger *zap.Logger) *GoogleOauth2 {
	if logger == nil {
		logger = zap.NewNop()
	}

	oauth2Config := oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  "",
		Endpoint:     google.Endpoint,
	}

	return &GoogleOauth2{
		config: oauth2Config,
		logger: logger,
	}
}

func (o *GoogleOauth2) ValidCode(ctx context.Context, code, redirectURI, codeVerifier string) error {
	authCodeVerify := oauth2.SetAuthURLParam("code_verifier", codeVerifier)
	authRedirectUrl := oauth2.SetAuthURLParam("redirect_uri", redirectURI)
	_, err := o.config.Exchange(ctx, code, oauth2.AccessTypeOffline, authCodeVerify, authRedirectUrl)
	if err != nil {
		return fmt.Errorf("there was an error trying to exchange the code for the token")
	}

	return nil
}

func (o *GoogleOauth2) ClientID() string {
	return o.config.ClientID
}

func (o *GoogleOauth2) Endpoint() string {
	return o.config.Endpoint.AuthURL
}
