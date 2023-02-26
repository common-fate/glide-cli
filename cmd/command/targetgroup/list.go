package targetgroup

import (
	"os"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var ListCommand = cli.Command{
	Name:        "list",
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

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Target Schema"})
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetBorder(false)

		for _, d := range res.JSON200.TargetGroups {
			table.Append([]string{
				d.Id, d.TargetSchema.From,
			})
		}
		table.Render()

		return nil
	}),
}
