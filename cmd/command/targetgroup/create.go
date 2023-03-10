package targetgroup

import (
	"fmt"
	"net/http"

	"github.com/AlecAivazis/survey/v2"
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/prompt"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/provider-registry-sdk-go/pkg/registryclient"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Description: "Create a target group",
	Usage:       "Create a target group",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id"},
		&cli.StringFlag{Name: "schema-from", Usage: "publisher/name@version/kind"},
		&cli.BoolFlag{Name: "ok-if-exists", Value: false},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context

		id := c.String("id")
		if id == "" {
			err := survey.AskOne(&survey.Input{Message: "Enter an ID for the target group"}, &id)
			if err != nil {
				return err
			}
		}

		schemaFrom := c.String("schema-from")
		if schemaFrom == "" {
			registry, err := registryclient.New(ctx)
			if err != nil {
				return errors.Wrap(err, "configuring provider registry client")
			}
			provider, err := prompt.Provider(ctx, registry)
			if err != nil {
				return err
			}
			kind, err := prompt.Kind(*provider)
			if err != nil {
				return err
			}
			schemaFrom = provider.String() + "/" + kind
			clio.Infof("Using schema from %s", schemaFrom)
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		res, err := cf.AdminCreateTargetGroupWithResponse(ctx, types.AdminCreateTargetGroupJSONRequestBody{
			Id:           id,
			TargetSchema: schemaFrom,
		})
		if err != nil {
			return err
		}
		switch res.StatusCode() {
		case http.StatusCreated:
			clio.Successf("Successfully created the targetgroup: %s", id)
		case http.StatusConflict:
			// if ok-if-exists flag is provided then gracefully return no error.
			if c.Bool("ok-if-exists") {
				clio.Infof("Targetgroup with that ID already exists: '%s'", id)

				return nil
			}

			return clierr.New(fmt.Sprintf("Duplicate targetgroup ID provided. Targetgroup with that ID '%s' already exist", id))
		case http.StatusUnauthorized:
			return errors.New(res.JSON401.Error)
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}
		return nil

	},
}
