package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

var jwtCfg = middleware.JWTConfig{Key: []byte(testutil.SigningKey), Issuer: testutil.Issuer, Audience: testutil.Audience}

func TestIssue_HasExpectedClaims(t *testing.T) {
	now := time.Now()
	access, err := NewIssuer(jwtCfg, 15*time.Minute).Issue("u1", "s1", []string{middleware.RoleConsumer}, now)
	require.NoError(t, err)

	claims, err := middleware.NewAccessParser(jwtCfg).Parse(access.Raw)

	require.NoError(t, err)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "s1", claims.SessionID)
	assert.Equal(t, access.JTI, claims.ID)
	assert.Equal(t, []string{middleware.RoleConsumer}, claims.Roles)
	assert.Equal(t, testutil.Issuer, claims.Issuer)
	assert.Equal(t, jwt.ClaimStrings{testutil.Audience}, claims.Audience)
	assert.WithinDuration(t, now.Add(15*time.Minute), claims.ExpiresAt.Time, time.Second)
	assert.Equal(t, access.ExpiresAt, claims.ExpiresAt.Time.UTC().Truncate(time.Second))
}

func TestParse_AlgNoneIsRejected(t *testing.T) {
	_, err := middleware.NewAccessParser(jwtCfg).Parse(testutil.AlgNoneToken(t))
	assert.Error(t, err)
}

func TestParse_ForeignSignatureIsRejected(t *testing.T) {
	foreign := middleware.JWTConfig{Key: []byte(strings.Repeat("x", 32)), Issuer: testutil.Issuer, Audience: testutil.Audience}
	access, err := NewIssuer(foreign, time.Minute).Issue("u1", "s1", nil, time.Now())
	require.NoError(t, err)

	_, err = middleware.NewAccessParser(jwtCfg).Parse(access.Raw)

	assert.Error(t, err)
}

func TestParse_ExpiredIsRejected(t *testing.T) {
	access, err := NewIssuer(jwtCfg, time.Minute).Issue("u1", "s1", nil, time.Now().Add(-2*time.Minute))
	require.NoError(t, err)

	_, err = middleware.NewAccessParser(jwtCfg).Parse(access.Raw)

	assert.Error(t, err)
}

func TestHashOpaque_IsDeterministicSHA256(t *testing.T) {
	assert.Equal(t, HashOpaque("abc"), HashOpaque("abc"))
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", HashOpaque("abc"))
	assert.NotEqual(t, HashOpaque("abc"), HashOpaque("abd"))
}

func TestNewOpaque_IsRandomAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		raw, hash, err := NewOpaque()
		require.NoError(t, err)
		assert.Len(t, raw, 43, "32 bytes en base64url sin relleno")
		assert.Equal(t, HashOpaque(raw), hash)
		assert.False(t, seen[raw], "token repetido")
		seen[raw] = true
	}
	family, err := newFamilyID()
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, family)
}
