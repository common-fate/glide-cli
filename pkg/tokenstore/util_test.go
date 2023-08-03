package tokenstore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"golang.org/x/oauth2"
)

func TestShouldRefresh(t *testing.T) {
	type testcase struct {
		name          string
		token         oauth2.Token
		shouldRefresh bool
	}

	testcases := []testcase{
		{
			name: "24 hours away passes",
			token: oauth2.Token{
				Expiry: time.Now().Add(time.Hour * 24),
			},
			shouldRefresh: false,
		},
		{
			name: "2 minutes away refreshes",
			token: oauth2.Token{
				Expiry: time.Now().Add(time.Minute * 2),
			},
			shouldRefresh: true,
		},
		{
			name: "5 minutes away refreshes",
			token: oauth2.Token{
				Expiry: time.Now().Add(time.Minute * 5),
			},
			shouldRefresh: true,
		},
		{
			name: "5 minutes after refreshes",
			token: oauth2.Token{
				Expiry: time.Now().Add(time.Minute * -5),
			},
			shouldRefresh: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			now := time.Now()

			outcome := ShouldRefreshToken(tc.token, now)

			assert.Equal(t, outcome, tc.shouldRefresh)

		})
	}

}
