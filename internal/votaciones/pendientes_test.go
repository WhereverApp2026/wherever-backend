package votaciones

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
)

func TestPuedeVotarEn_InvitadaConfirmadaSinSorpresa(t *testing.T) {
	me, otra := bson.NewObjectID(), bson.NewObjectID()
	now := time.Now()
	grupo := func(estado string, sorpresa *grupos.Surprise) grupos.Group {
		return grupos.Group{Members: []grupos.Member{{UserID: me, Role: grupos.RoleMember, Status: estado}}, Surprise: sorpresa}
	}
	assert.True(t, PuedeVotarEn(grupo(grupos.StatusInvited, nil), me, now))
	assert.True(t, PuedeVotarEn(grupo(grupos.StatusConfirmed, nil), me, now))
	assert.False(t, PuedeVotarEn(grupo(grupos.StatusDeclined, nil), me, now), "quien declinó no vota")
	assert.False(t, PuedeVotarEn(grupo(grupos.StatusConfirmed, nil), otra, now), "quien no está en el grupo")
	oculta := &grupos.Surprise{Active: true, For: me, RevealAt: now.Add(time.Hour)}
	assert.False(t, PuedeVotarEn(grupo(grupos.StatusConfirmed, oculta), me, now), "la sorpresa le oculta las paradas")
	assert.True(t, PuedeVotarEn(grupo(grupos.StatusConfirmed, oculta), me, now.Add(2*time.Hour)), "ya revelada")
}
