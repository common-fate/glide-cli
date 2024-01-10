package command

import (
	"github.com/common-fate/clio"
	"github.com/common-fate/glide-cli/pkg/config"
	"github.com/common-fate/glide-cli/pkg/tokenstore"
	"github.com/urfave/cli/v2"
)

var Logout = cli.Command{
	Name:  "logout",
	Usage: "Log out of Common Fate",
	Action: func(c *cli.Context) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ts := tokenstore.New(cfg.CurrentContext)
		err = ts.Clear()
		if err != nil {
			return err
		}

		clio.Success("logged out")

		return nil
	},
}
