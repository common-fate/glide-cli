package handler

import (
	"errors"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "handler",
	Description: "Manage handlers",
	Usage:       "Manage handlers",
	Subcommands: []*cli.Command{
		&RegisterCommand,
		&ValidateCommand,
		&ListCommand,
		&DiagnosticCommand,
		&LogsCommand,
		&DeleteCommand,
	},
}

/*


```bash
# register the deployment with Common Fate
> cfcli deployment register --runtime=aws-lambda --id=okta-1 --aws-region=us-east-1 --aws-account=123456789012
[✔] registered deployment 'okta-1' with Common Fate
```

to exec this same command with go run, you can do:
go run cf/cmd/cli/main.go deployment register --runtime=aws-lambda --id=okta-1 --aws-region=us-east-1 --aws-account=123456789012

*/

var RegisterCommand = cli.Command{
	Name:        "register",
	Description: "Register a handler in Common Fate",
	Usage:       "Register a handler in Common Fate",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true},
		&cli.StringFlag{Name: "runtime", Required: true, Value: "aws-lambda"},
		&cli.StringFlag{Name: "aws-region", Required: true},
		&cli.StringFlag{Name: "aws-account", Required: true},
	},
	Action: func(c *cli.Context) error {

		ctx := c.Context

		reqBody := types.AdminRegisterHandlerJSONRequestBody{
			AwsAccount: c.String("aws-account"),
			AwsRegion:  c.String("aws-region"),
			Runtime:    c.String("runtime"),
			Id:         c.String("id"),
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		result, err := cf.AdminRegisterHandlerWithResponse(ctx, reqBody)
		if err != nil {
			return err
		}

		switch result.StatusCode() {
		case 201:
			clio.Successf("Successfully registered handler '%s' with Common Fate", c.String("id"))
			return nil
		default:
			return errors.New(string(result.Body))
		}
	},
}
