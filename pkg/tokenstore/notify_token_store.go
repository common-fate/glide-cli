package tokenstore

import (
	"sync"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// TokenNotifyFunc is a function that accepts an oauth2 Token upon refresh, and
// returns an error if it should not be used.
type TokenNotifyFunc func(*oauth2.Token) error

// NotifyRefreshTokenSource is essentially `oauth2.ResuseTokenSource` with `TokenNotifyFunc` added.
type NotifyRefreshTokenSource struct {
	New       oauth2.TokenSource
	mu        sync.Mutex // guards t
	T         *oauth2.Token
	SaveToken TokenNotifyFunc // called when token refreshed so new refresh token can be persisted
}

func StoreNewToken(t *oauth2.Token) error {
	// persist token
	return nil // or error
}

// Token returns the current token if it's still valid, else will
// refresh the current token (using r.Context for HTTP client
// information) and return the new one.
func (s *NotifyRefreshTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// if s.T.Valid() {
	// 	zap.S().Debug("returning cached in-memory token")
	// 	return s.T, nil
	// }
	zap.S().Debug("refreshing token")
	t, err := s.New.Token()
	if err != nil {
		return nil, err
	}
	s.T = t
	return t, s.SaveToken(t)
}
