package handler

import (
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/prompt"
	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Description: "Delete handlers",
	Usage:       "Delete handlers",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id"},
	},
	Action: cli.ActionFunc(func(c *cli.Context) error {
		ctx := c.Context
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}
		id := c.String("id")
		if id == "" {
			h, err := prompt.Handler(ctx, cf)
			if err != nil {
				return err
			}
			id = h.Id
		}
		_, err = cf.AdminDeleteHandlerWithResponse(ctx, id)
		if err != nil {
			return err
		}
		clio.Success("Deleted handler ", c.String("id"))

		return nil
	}),
}
