package sso

import (
	"fmt"
	"net/http"
	"os"

	"github.com/common-fate/cli/pkg/awscfg"
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/urfave/cli/v2"
	"gopkg.in/ini.v1"
)

var generate = cli.Command{
	Name:  "generate",
	Usage: "Generate AWS config with available Access Rules, printing to stdout",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "sso-start-url", Required: true, Usage: "the AWS SSO start URL (e.g. https://d-123456abc.awsapps.com/start)"},
		&cli.StringFlag{Name: "sso-region", Required: true, Usage: "the AWS SSO region"},
		&cli.StringFlag{Name: "prefix", Usage: "Specify a prefix for all generated profile names"},
		&cli.BoolFlag{Name: "no-credential-process", Usage: "Generate profiles without the Granted credential-process integration"},
		&cli.StringFlag{Name: "profile-template", Usage: "Specify profile name template", Value: "{{ .AccountName }}/{{ .RoleName }}"},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		depCtx, err := cfg.Current()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}
		rules, err := cf.UserListAccessRulesWithResponse(ctx)
		if err != nil {
			return err
		}
		if rules.StatusCode() != http.StatusOK {
			return fmt.Errorf("%s", string(rules.Body))
		}

		var profiles []awscfg.SSOProfile

		for _, r := range rules.JSON200.AccessRules {
			ruleDetail, err := cf.UserGetAccessRuleWithResponse(ctx, r.ID)
			if err != nil {
				return err
			}

			// only rules for the aws-sso Access Provider are relevant here
			if ruleDetail.JSON200.Target.Provider.Type != "aws-sso" {
				continue
			}

			accountId := ruleDetail.JSON200.Target.Arguments.AdditionalProperties["accountId"]
			permissionSetArn := ruleDetail.JSON200.Target.Arguments.AdditionalProperties["permissionSetArn"]

			// add all options to our profile map
			for _, acc := range accountId.Options {
				for _, ps := range permissionSetArn.Options {
					p := awscfg.SSOProfile{
						AccountId:     acc.Value,
						AccountName:   acc.Label,
						RoleName:      ps.Label,
						StartUrl:      c.String("sso-start-url"),
						SSORegion:     c.String("sso-region"),
						CommonFateURL: depCtx.DashboardURL,
					}
					profiles = append(profiles, p)
				}
			}
		}

		config := ini.Empty()
		err = awscfg.Merge(awscfg.MergeOpts{
			Config:              config,
			Prefix:              c.String("prefix"),
			SectionNameTemplate: c.String("profile-template"),
			Profiles:            profiles,
			NoCredentialProcess: c.Bool("no-credential-process"),
		})
		if err != nil {
			return err
		}
		_, err = config.WriteTo(os.Stdout)
		return err
	},
}
