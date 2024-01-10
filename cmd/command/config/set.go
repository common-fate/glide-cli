package config

import (
	"fmt"
	"strings"

	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/glide-cli/pkg/config"
	"github.com/urfave/cli/v2"
)

var set = cli.Command{
	Name:  "set",
	Usage: "set a config variable in ~/.commonfate/config",
	Action: func(c *cli.Context) error {
		if c.Args().Len() != 2 {
			return clierr.New("usage: cf oss config set [key] [value]")
		}

		key := c.Args().Get(0)
		val := c.Args().Get(1)

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		current, err := cfg.Current()
		if err != nil {
			return err
		}

		switch key {
		case "api_url":
			current.APIURL = val
		case "dashboard_url":
			current.DashboardURL = val
		default:
			return fmt.Errorf("unknown key %s. supported keys: %s", key, strings.Join(config.Keys, ", "))
		}

		cfg.Contexts[cfg.CurrentContext] = *current

		err = config.Save(cfg)
		if err != nil {
			return err
		}

		return nil
	},
}
