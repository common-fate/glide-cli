package profilesource

import (
	"context"

	"github.com/common-fate/awsconfigfile"
	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
)

// Source reads available AWS SSO profiles from the Common Fate API.
// It implements the awsconfigfile.Source interface
type Source struct {
	SSORegion string
	StartURL  string
}

func (s Source) GetProfiles(ctx context.Context) ([]awsconfigfile.SSOProfile, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	depCtx, err := cfg.Current()
	if err != nil {
		return nil, err
	}

	cf, err := client.FromConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	clio.Infof("listing available profiles from Common Fate (%s)", cfg.CurrentOrEmpty().DashboardURL)

	rules, err := cf.UserListAccessRulesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	var profiles []awsconfigfile.SSOProfile

	for _, r := range rules.JSON200.AccessRules {
		ruleDetail, err := cf.UserGetAccessRuleWithResponse(ctx, r.ID)
		if err != nil {
			return nil, err
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
				p := awsconfigfile.SSOProfile{
					AccountID:     acc.Value,
					AccountName:   acc.Label,
					RoleName:      ps.Label,
					SSOStartURL:   s.StartURL,
					SSORegion:     s.SSORegion,
					GeneratedFrom: "commonfate",
					CommonFateURL: depCtx.DashboardURL,
				}
				profiles = append(profiles, p)
			}
		}
	}
	return profiles, nil
}
