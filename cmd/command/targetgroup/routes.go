package targetgroup

import (
	"fmt"
	"os"
	"strconv"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var RoutesCommand = cli.Command{
	Name:        "routes",
	Description: "Manage Target Groups Routes",
	Usage:       "Manage Target Groups Routes",
	Subcommands: []*cli.Command{
		&ListRoutesCommand,
	},
}

var ListRoutesCommand = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Description: "List target group routes",
	Usage:       "List target group routes",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true},
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

		res, err := cf.AdminListTargetRoutesWithResponse(ctx, c.String("id"))
		if err != nil {
			return err
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Target Group Id", "Handler Id", "Kind", "Priority", "Valid", "Diagnostics"})
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetBorder(false)

		for _, d := range res.JSON200.Routes {
			table.Append([]string{
				d.TargetGroupId, d.HandlerId, d.Kind, strconv.Itoa(d.Priority), strconv.FormatBool(d.Valid), fmt.Sprintf("%v", d.Diagnostics),
			})
		}
		table.Render()

		return nil
	}),
}
