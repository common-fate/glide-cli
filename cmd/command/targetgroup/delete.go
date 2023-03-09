package targetgroup

import (
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Description: "Delete a target group",
	Usage:       "Delete a target group",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		id := c.String("id")

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		_, err = cf.AdminDeleteTargetGroupWithResponse(ctx, id)
		if err != nil {
			return err
		}

		clio.Successf("Deleted target group %s", id)

		return nil
	},
}
