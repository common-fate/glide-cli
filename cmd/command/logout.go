package command

import (
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/tokenstore"
	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

func Logout(opts ...func(*LoginOpts)) *cli.Command {
	var o LoginOpts
	for _, opt := range opts {
		opt(&o)
	}

	cmd := cli.Command{
		Name:  "logout",
		Usage: "Log out of Common Fate",
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			ts := tokenstore.New(cfg.CurrentContext, tokenstore.WithKeyring(o.Keyring))
			err = ts.Clear()
			if err != nil {
				return err
			}

			clio.Success("logged out")

			return nil
		},
	}
	return &cmd
}
