package handler

import (
	"errors"
	"net/http"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/urfave/cli/v2"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Description: "Delete handlers",
	Usage:       "Delete handlers",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true},
	},
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
		res, err := cfApi.AdminDeleteHandlerWithResponse(ctx, c.String("id"))
		if err != nil {
			return err
		}
		switch res.StatusCode() {
		case http.StatusNoContent:
			clio.Success("Deleted handler ", c.String("id"))
		case http.StatusUnauthorized:
			return errors.New(res.JSON401.Error)
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}
		return nil
	}),
}
