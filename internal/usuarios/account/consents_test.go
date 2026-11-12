package account

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

var vigentes = Versions{Terms: "2026-10", Notice: "2026-11"}

func TestConsentCurrent_NeedsBothVersions(t *testing.T) {
	s := (&Service{}).WithConsents(nil, vigentes)
	for name, tc := range map[string]struct {
		p    usuarios.Privacy
		want bool
	}{
		"las dos vigentes":        {usuarios.Privacy{TermsVersion: "2026-10", NoticeVersion: "2026-11"}, true},
		"términos viejos":         {usuarios.Privacy{TermsVersion: "2026-09", NoticeVersion: "2026-11"}, false},
		"aviso viejo":             {usuarios.Privacy{TermsVersion: "2026-10", NoticeVersion: "2026-10"}, false},
		"cuenta previa sin aviso": {usuarios.Privacy{TermsVersion: "2026-10"}, false},
	} {
		assert.Equal(t, tc.want, s.consentCurrent(tc.p), name)
		got := s.privacyOf(tc.p)
		require.NotNil(t, got.ConsentCurrent, name)
		assert.Equal(t, tc.want, *got.ConsentCurrent, name)
	}
}

func TestConsentCurrent_FalseWithoutVersions(t *testing.T) {
	assert.False(t, (&Service{}).consentCurrent(usuarios.Privacy{}))
}

func TestStaleVersions_ReportsEachField(t *testing.T) {
	s := (&Service{}).WithConsents(nil, vigentes)
	assert.Empty(t, s.staleVersions(ConsentRequest{TermsVersion: "2026-10", NoticeVersion: "2026-11"}))
	fields := s.staleVersions(ConsentRequest{TermsVersion: "2026-09", NoticeVersion: "2026-01"})
	require.Len(t, fields, 2)
	assert.Equal(t, "version_terminos", fields[0].Field)
	assert.Equal(t, "version_aviso_privacidad", fields[1].Field)
}

func TestAcceptConsent_StaleVersionIs422BeforeTouchingTheBase(t *testing.T) {
	// Sin repositorio de usuarios: si llegara a la base, entraría en pánico.
	s := (&Service{}).WithConsents(&usuarios.ConsentRepository{}, vigentes)
	_, err := s.AcceptConsent(context.Background(), &middleware.Claims{}, ConsentRequest{TermsVersion: "2026-09", NoticeVersion: "2026-11"})
	var e *apperr.Error
	require.True(t, errors.As(err, &e))
	assert.Equal(t, apperr.KindValidation, e.Kind)
}

func TestAcceptConsent_FailsClosedWithoutRepository(t *testing.T) {
	_, err := (&Service{}).AcceptConsent(context.Background(), &middleware.Claims{}, ConsentRequest{TermsVersion: "2026-10", NoticeVersion: "2026-11"})
	var e *apperr.Error
	require.True(t, errors.As(err, &e))
	assert.Equal(t, apperr.KindUnavailable, e.Kind)
}

func TestProfileResponse_ExportOmitsConsentCurrent(t *testing.T) {
	assert.Nil(t, profileResponse(usuarios.User{}).Privacy.ConsentCurrent)
	s := (&Service{}).WithConsents(nil, vigentes)
	assert.NotNil(t, s.profileOf(usuarios.User{}).Privacy.ConsentCurrent)
}
