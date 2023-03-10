package targetgroup

import (
	"os"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/table"
	"github.com/urfave/cli/v2"
)

var ListCommand = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Description: "List target groups",
	Usage:       "List target groups",
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

		res, err := cf.AdminListTargetGroupsWithResponse(ctx)
		if err != nil {
			return err
		}
		tbl := table.New(os.Stderr)
		tbl.Columns("ID", "Target Schema")
		for _, targetGroup := range res.JSON200.TargetGroups {
			tbl.Row(targetGroup.Id, targetGroup.TargetSchema.From)
		}
		return tbl.Flush()

	}),
}
