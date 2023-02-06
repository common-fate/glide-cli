package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/tokenstore"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// ErrorHandlingClient checks the response status code
// and creates an error if the API returns greater than 300.
type ErrorHandlingClient struct {
	Client       *http.Client
	LoginCommand string
}

func (rd *ErrorHandlingClient) Do(req *http.Request) (*http.Response, error) {
	res, err := rd.Client.Do(req)
	var ne *url.Error
	if errors.As(err, &ne) && ne.Err == tokenstore.ErrNotFound {
		return nil, clierr.New(fmt.Sprintf("To get started with Common Fate, please run: '%s'", rd.LoginCommand))
	}
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 300 {
		// response is ok
		return res, nil
	}

	// if we get here, the API has returned an error
	// surface this as a Go error so we don't need to handle it everywhere in our CLI codebase.
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res, errors.Wrap(err, "reading error response body")
	}

	e := clierr.New(fmt.Sprintf("Common Fate API returned an error (code %v): %s", res.StatusCode, string(body)))

	if res.StatusCode == http.StatusUnauthorized {
		e.Messages = append(e.Messages, clierr.Infof("To log in to Common Fate, run: '%s'", rd.LoginCommand))
	}

	return res, e
}

// FromConfig creates a new client from a Common Fate CLI config.
// The client loads the OAuth2.0 tokens from the system keychain.
// The client automatically refreshes the access token if it is expired.
func FromConfig(ctx context.Context, cfg *config.Config) (*types.ClientWithResponses, error) {
	depCtx, err := cfg.Current()
	if err != nil {
		return nil, err
	}
	exp, err := depCtx.FetchExports(ctx) // fetch the aws-exports.json file containing the exported URLs
	if err != nil {
		return nil, err
	}

	return New(ctx, exp.APIURL, cfg.CurrentContext)
}

// New creates a new client, specifying the URL and context directly.
// The client loads the OAuth2.0 tokens from the system keychain.
// The client automatically refreshes the access token if it is expired.
func New(ctx context.Context, server, context string) (*types.ClientWithResponses, error) {
	ts := tokenstore.New(context)
	oauthClient := oauth2.NewClient(ctx, &ts)
	httpClient := &ErrorHandlingClient{Client: oauthClient, LoginCommand: "cf login"}

	return types.NewClientWithResponses(server, types.WithHTTPClient(httpClient))
}
