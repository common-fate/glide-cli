package command

import (
	"fmt"
	"os"

	"github.com/common-fate/cli/pkg/client"
	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/table"
	"github.com/common-fate/clio"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/urfave/cli/v2"
)

type Resource struct {
	ProviderID  string
	Properties  map[string]Property
	AccessRules accessRule
}

type Property struct {
	Label string
	Value string
}

type accessRule struct {
	ID   string
	Name string
}

// a map to lookup labels for arguments
// structure is provider ID -> argument type -> argument value -> argument label
// e.g. aws-sso -> permissionSetArn -> ps::12345 -> AdminAccess
type argLabelLookup struct {
	labels map[string]map[string]map[string]string
}

func (all *argLabelLookup) AddLabel(providerID, argName, argValue, argLabel string) {
	if all.labels == nil {
		all.labels = map[string]map[string]map[string]string{}
	}

	if _, ok := all.labels[providerID]; !ok {
		all.labels[providerID] = map[string]map[string]string{}
	}

	if _, ok := all.labels[providerID][argName]; !ok {
		all.labels[providerID][argName] = map[string]string{}
	}

	all.labels[providerID][argName][argValue] = argLabel
}

func (all *argLabelLookup) Get(providerID, argName, argValue string) string {
	if all.labels == nil {
		return ""
	}

	if _, ok := all.labels[providerID]; !ok {
		return ""
	}

	if _, ok := all.labels[providerID][argName]; !ok {
		return ""
	}

	if _, ok := all.labels[providerID][argName][argValue]; !ok {
		return ""
	}

	return all.labels[providerID][argName][argValue]
}

var Get = cli.Command{
	Name:  "get",
	Usage: "Get resources",
	Action: func(c *cli.Context) error {
		ctx := c.Context

		cfg, err := config.Load()
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

		clio.Infow("rules", "rules", rules.JSON200.AccessRules)

		var resources []Resource

		var labelLookup argLabelLookup

		for _, r := range rules.JSON200.AccessRules {
			clio.Infof("processing " + r.ID)
			details, err := cf.UserGetAccessRuleWithResponse(ctx, r.ID)
			if err != nil {
				return err
			}

			cra := types.CreateRequestWith{
				AdditionalProperties: map[string][]string{},
			}

			for k, v := range details.JSON200.Target.Arguments.AdditionalProperties {
				// the values of the particular option - e.g ps::1234, ps::4542345
				var values []string
				for _, opt := range v.Options {
					labelLookup.AddLabel(r.Target.Provider.Id, k, opt.Value, opt.Label)
					values = append(values, opt.Value)
				}
				cra.AdditionalProperties[k] = values

			}

			combinations, err := cra.ArgumentCombinations()
			if err != nil {
				return err
			}
			for _, combination := range combinations {
				properties := map[string]Property{}
				for k, v := range combination {
					properties[k] = Property{
						Value: v,
						Label: labelLookup.Get(r.Target.Provider.Id, k, v),
					}
				}

				res := Resource{
					ProviderID: details.JSON200.Target.Provider.Id,
					Properties: properties,
					AccessRules: accessRule{
						ID:   r.ID,
						Name: r.Name,
					},
				}
				resources = append(resources, res)
			}
		}

		allProperties := map[string]bool{}
		for _, r := range resources {
			for k := range r.Properties {
				allProperties[k] = true
			}
		}

		var propertyColumns []string
		for p := range allProperties {
			propertyColumns = append(propertyColumns, p)
		}

		columns := []string{"provider"}
		columns = append(columns, propertyColumns...)

		w := table.New(os.Stdout)
		w.Columns(columns...)

		for _, r := range resources {
			row := []string{r.ProviderID}

			for _, p := range propertyColumns {
				if v, ok := r.Properties[p]; ok {
					row = append(row, formatProperty(p, v))
				} else {
					row = append(row, "")
				}

			}

			w.Row(row...)
		}

		w.Flush()

		return nil
	},
}

func formatProperty(propertyName string, p Property) string {
	if propertyName == "permissionSetArn" {
		return p.Label
	}
	return fmt.Sprintf("%s (%s)", p.Label, p.Value)
}
