package handler

import (
	"os"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var ListCommand = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Description: "List handlers",
	Usage:       "List handlers",
	Action: cli.ActionFunc(func(c *cli.Context) error {
		ctx := c.Context
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cfApi, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}
		res, err := cfApi.AdminListHandlersWithResponse(ctx)
		if err != nil {
			return err
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Account", "Region", "Health"})
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetBorder(false)

		for _, d := range res.JSON200.Res {
			health := "healthy"
			if !d.Healthy {
				health = "unhealthy"
			}
			table.Append([]string{
				d.Id, d.AwsAccount, d.AwsRegion, health,
			})
		}
		table.Render()

		return nil
	}),
}
