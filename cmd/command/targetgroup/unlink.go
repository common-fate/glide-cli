package targetgroup

import (
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/urfave/cli/v2"
)

var UnlinkCommand = cli.Command{
	Name:        "unlink",
	Description: "Unlink a deployment from a target group",
	Usage:       "Unlink a deployment from a target group",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "deployment", Required: true},
		&cli.StringFlag{Name: "target-group", Required: true},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		_, err = cf.AdminRemoveTargetGroupLinkWithResponse(ctx, c.String("target-group"), &types.AdminRemoveTargetGroupLinkParams{
			DeploymentId: c.String("deployment"),
		})
		if err != nil {
			return err
		}

		clio.Successf("Unlinked deployment %s from group %s", c.String("deployment"), c.String("target-group"))

		return nil
	},
}
