package auth_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"gojira/internal/auth"
	"gojira/internal/domain"
)

func TestParseKeepsConcurrentClaimsIsolated(t *testing.T) {
	type tokenCase struct {
		token    string
		wantID   int64
		wantName string
		wantRole string
	}
	type result struct {
		tokenCase
		claims *auth.Claims
		err    error
	}

	service := &auth.Service{Secret: []byte("gojira-dev-secret-change-in-prod-32b")}
	roles := []string{domain.RoleAdmin, domain.RolePM, domain.RoleDev, domain.RoleQA}
	cases := make([]tokenCase, 16)
	for i := range cases {
		user := &domain.User{
			ID:       int64(i + 1),
			Username: fmt.Sprintf("concurrent-user-%02d", i+1),
			Role:     roles[i%len(roles)],
		}
		token, err := service.Sign(user, "access", time.Hour)
		if err != nil {
			t.Fatalf("sign token %d: %v", i, err)
		}
		cases[i] = tokenCase{token: token, wantID: user.ID, wantName: user.Username, wantRole: user.Role}
	}

	start := make(chan struct{})
	results := make(chan result, len(cases))
	var ready sync.WaitGroup
	var done sync.WaitGroup
	for _, tc := range cases {
		ready.Add(1)
		done.Add(1)
		go func(tc tokenCase) {
			defer done.Done()
			ready.Done()
			<-start
			claims, err := service.Parse(tc.token)
			results <- result{tokenCase: tc, claims: claims, err: err}
		}(tc)
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(results)

	for got := range results {
		if got.err != nil {
			t.Errorf("parse token for %s: %v", got.wantName, got.err)
			continue
		}
		if got.claims.UserID != got.wantID || got.claims.Username != got.wantName || got.claims.Role != got.wantRole || got.claims.Kind != "access" {
			t.Errorf("token for %s resolved to uid=%d username=%q role=%q kind=%q", got.wantName, got.claims.UserID, got.claims.Username, got.claims.Role, got.claims.Kind)
		}
	}
}
