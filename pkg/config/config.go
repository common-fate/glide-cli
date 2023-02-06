package config

import (
	"fmt"

	"github.com/common-fate/clio/clierr"
)

type Config struct {
	CurrentContext string             `toml:"current_context" json:"current_context"`
	Contexts       map[string]Context `toml:"context" json:"context"`
}

type Context struct {
	AuthURL      string `toml:"auth_url"`
	TokenURL     string `toml:"token_url"`
	DashboardURL string `toml:"dashboard_url" json:"dashboard_url"`
	APIURL       string `toml:"api_url" json:"api_url"`
	ClientID     string `toml:"client_id" json:"client_id"`
}

// Current loads the current context as specified in the 'current_context' field in the config file.
// It returns an error if there are no contexts, or if the 'current_context' field doesn't match
// any contexts defined in the config file.
func (c Config) Current() (*Context, error) {
	if c.Contexts == nil {
		return nil, clierr.New("No contexts were found in Common Fate config file.", clierr.Infof("To log in to Common Fate, run: 'cf login'"))
	}

	got, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return nil, clierr.New(fmt.Sprintf("Could not find context '%s' in Common Fate config file", c.CurrentContext), clierr.Infof("To log in to Common Fate, run: 'cf login'"))
	}

	return &got, nil
}

// Default returns an empty config.
func Default() *Config {
	return &Config{
		CurrentContext: "",
		Contexts:       map[string]Context{},
	}
}
