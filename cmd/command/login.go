package command

import (
	"errors"
	"net/http"

	"github.com/common-fate/cli/pkg/authflow"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/tokenstore"
	"github.com/common-fate/clio"
	"github.com/pkg/browser"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var Login = cli.Command{
	Name:  "login",
	Usage: "Log in to Common Fate",
	Action: func(c *cli.Context) error {
		url := c.Args().First()
		if url == "" {
			return errors.New("usage: cf login <DASHBOARD URL>")
		}

		ctx := c.Context

		cfg, err := config.Load()
		if err != nil {
			return err
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

		ts := tokenstore.New(cfg.CurrentContext)

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
			cfg.Contexts["default"] = res.Context
			err = config.Save(cfg)
			if err != nil {
				return err
			}

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
			clio.Infof("Opening your browser to %s", u)
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
	},
}
