package handler

import (
	"fmt"
	"os"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var DiagnosticCommand = cli.Command{
	Name:        "diagnostic",
	Description: "List diagnostic logs for a handler",
	Usage:       "List diagnostic logs for a handler",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true},
	},
	Action: cli.ActionFunc(func(c *cli.Context) error {
		ctx := c.Context
		id := c.String("id")
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}
		res, err := cf.AdminGetHandlerWithResponse(ctx, id)
		if err != nil {
			return err
		}

		health := "healthy"
		if !res.JSON200.Healthy {
			health = "unhealthy"
		}
		fmt.Println("Diagnostic Logs:")
		fmt.Printf("%s %s %s %s\n", res.JSON200.Id, res.JSON200.AwsAccount, res.JSON200.AwsRegion, health)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Level", "Message"})
		table.SetAutoWrapText(false)
		table.SetAutoFormatHeaders(true)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetBorder(false)

		for _, d := range res.JSON200.Diagnostics {
			table.Append([]string{
				d.Message,
			})
		}
		table.Render()

		return nil
	}),
}
