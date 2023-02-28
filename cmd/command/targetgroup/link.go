package targetgroup

import (
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/urfave/cli/v2"
)

var LinkCommand = cli.Command{
	Name:        "link",
	Description: "Link a handler to a target group",
	Usage:       "Link a handler to a target group",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "target-group", Required: true},
		&cli.StringFlag{Name: "handler", Required: true},
		&cli.StringFlag{Name: "kind", Required: true},
		&cli.IntFlag{Name: "priority", Value: 100},
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

		_, err = cf.AdminCreateTargetGroupLinkWithResponse(ctx, c.String("target-group"), types.AdminCreateTargetGroupLinkJSONRequestBody{
			DeploymentId: c.String("handler"),
			Priority:     c.Int("priority"),
			Kind:         c.String("kind"),
		})
		if err != nil {
			return err
		}

		clio.Successf("Successfully linked the handler '%s' with target group '%s' using kind: '%s'", c.String("handler"), c.String("target-group"), c.String("kind"))

		return nil
	},
}
