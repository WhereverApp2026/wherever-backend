package usuarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

// Una cuenta previa (sin `privacidad.social`) queda como hoy: actividad para amigos, todo
// compartido, en sugerencias y solicitudes de cualquiera (nv-82).
func TestSocial_ValoresPorOmision(t *testing.T) {
	var s Social
	assert.Equal(t, ActivityFriends, s.ActivityLevel())
	assert.Equal(t, RequestsAnyone, s.RequestsFrom())
	assert.False(t, s.HidesActivity())
	assert.True(t, s.SharesReviews())
	assert.True(t, s.SharesPlans())
	assert.True(t, s.SharesPostcards())
	reviews, plans, postcards, suggestions := s.Switches()
	assert.True(t, reviews && plans && postcards && suggestions)
}

// Un valor desconocido en la base cae al de por omisión, nunca a uno más abierto.
func TestSocial_ValorDesconocidoEsElDePorOmision(t *testing.T) {
	s := Social{Activity: "publico", Requests: "nadie"}
	assert.Equal(t, ActivityFriends, s.ActivityLevel())
	assert.Equal(t, RequestsAnyone, s.RequestsFrom())
	assert.Equal(t, ActivityAll, Social{Activity: ActivityAll}.ActivityLevel())
	assert.Equal(t, RequestsCode, Social{Requests: RequestsCode}.RequestsFrom())
}

// `actividad: nadie` apaga todo lo compartido aunque los interruptores sigan encendidos; los
// interruptores guardados no cambian.
func TestSocial_NadieOcultaTodo(t *testing.T) {
	s := Social{Activity: ActivityNobody, ShareReviews: ptr(true), SharePlans: ptr(true), SharePostcards: ptr(true)}
	assert.True(t, s.HidesActivity())
	assert.False(t, s.SharesReviews())
	assert.False(t, s.SharesPlans())
	assert.False(t, s.SharesPostcards())
	reviews, plans, postcards, _ := s.Switches()
	assert.True(t, reviews && plans && postcards)
}

// Cada interruptor apagado solo apaga lo suyo.
func TestSocial_InterruptoresIndependientes(t *testing.T) {
	assert.False(t, Social{ShareReviews: ptr(false)}.SharesReviews())
	assert.True(t, Social{ShareReviews: ptr(false)}.SharesPlans())
	assert.False(t, Social{SharePlans: ptr(false)}.SharesPlans())
	assert.True(t, Social{SharePlans: ptr(false)}.SharesPostcards())
	assert.False(t, Social{SharePostcards: ptr(false)}.SharesPostcards())
	_, _, _, suggestions := Social{InSuggestions: ptr(false)}.Switches()
	assert.False(t, suggestions)
	assert.True(t, Social{Activity: ActivityAll}.SharesPostcards())
}
