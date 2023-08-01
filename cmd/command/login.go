package command

import (
	"net/http"
	"time"

	"github.com/99designs/keyring"
	"github.com/AlecAivazis/survey/v2"
	"github.com/common-fate/cli/pkg/authflow"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/tokenstore"
	"github.com/common-fate/clio"
	"github.com/pkg/browser"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var Login = cli.Command{
	Name:   "login",
	Usage:  "Log in to Common Fate",
	Action: defaultLoginFlow.LoginAction,
}

// defaultLoginFlow is the login flow without any
// customisations to token storage.
var defaultLoginFlow LoginFlow

type LoginFlow struct {
	// Keyring optionally overrides the keyring that auth tokens are saved to.
	Keyring keyring.Keyring
	// ForceInteractive forces the survey prompt to appear
	ForceInteractive bool
}

func (lf LoginFlow) LoginAction(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var url string
	if !lf.ForceInteractive {
		// try and read the URL from the first provided argument
		url = c.Args().First()
	}

	if url == "" {

		prompt := &survey.Input{
			Message: "Your Common Fate dashboard URL",
			Default: cfg.CurrentOrEmpty().DashboardURL,
		}
		err = survey.AskOne(prompt, &url, survey.WithValidator(survey.Required))
		if err != nil {
			return err
		}
	}

	ctx := c.Context

	//check expire for current token if it exists
	ts := tokenstore.New(cfg.CurrentContext)

	token, err := ts.Token()
	if err != nil && err != tokenstore.ErrNotFound {
		return err

	}

	//what do we consider to be 'close to expiry' for now ill set it at 5 minutes?
	now := time.Now()
	timeDifference := (time.Minute * 5)
	//maybe we add a flag here to gate this as well
	if token.Expiry.Unix()-now.Unix() > int64(timeDifference) {
		//not within the range where we want to re-login
		clio.Infow("Auth token still valid, skipping login flow.")

		return nil
	}

	authResponse := make(chan authflow.Response)

	var g errgroup.Group

	authServer, err := authflow.FromDashboardURL(ctx, authflow.Opts{
		Response:     authResponse,
		DashboardURL: url,
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:    ":18900",
		Handler: authServer.Handler(),
	}

	// run the auth server on localhost
	g.Go(func() error {
		clio.Debugw("starting HTTP server", "address", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		clio.Debugw("auth server closed")
		return nil
	})

	// read the returned ID token from Cognito
	g.Go(func() error {
		res := <-authResponse

		err := server.Shutdown(ctx)
		if err != nil {
			return err
		}

		// check that the auth flow didn't error out
		if res.Err != nil {
			return err
		}

		// update the config file
		cfg.CurrentContext = "default"

		// is it a new URL if so, add it and reset config
		// otherwise it stays the same (which will preserve existing config; api_url)
		if cfg.Contexts["default"].DashboardURL != res.DashboardURL {
			cfg.Contexts["default"] = config.Context{
				DashboardURL: res.DashboardURL,
			}
		}

		err = config.Save(cfg)
		if err != nil {
			return err
		}

		ts := tokenstore.New(cfg.CurrentContext, tokenstore.WithKeyring(lf.Keyring))
		err = ts.Save(res.Token)
		if err != nil {
			return err
		}

		clio.Successf("logged in")

		return nil
	})

	// open the browser and read the token
	g.Go(func() error {
		u := "http://localhost:18900/auth/cognito/login"
		clio.Infof("Opening your web browser to: %s", u)
		err := browser.OpenURL(u)
		if err != nil {
			clio.Errorf("error opening browser: %s", err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	return nil
}
