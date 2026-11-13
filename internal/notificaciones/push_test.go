package notificaciones

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// at es una hora de hoy en México.
func at(hhmm string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-12 "+hhmm, negocios.MexicoCity())
	return t
}

func TestWanted_PreferenciasYHorarioSilencioso(t *testing.T) {
	groupsOn := usuarios.Preferences{Notifications: usuarios.Notifications{Groups: true}}
	groupsOff := usuarios.Preferences{}
	night := &usuarios.QuietHours{From: "22:00", To: "07:00"}
	afternoon := &usuarios.QuietHours{From: "14:00", To: "16:00"}
	withQuiet := func(q *usuarios.QuietHours) usuarios.Preferences {
		p := groupsOn
		p.QuietHours = q
		return p
	}
	cases := []struct {
		name  string
		prefs usuarios.Preferences
		kind  string
		now   time.Time
		want  bool
	}{
		{"grupos encendido", groupsOn, KindPlanInvite, at("12:00"), true},
		{"grupos apagado", groupsOff, KindVoteOpened, at("12:00"), false},
		{"amistad sin interruptor propio", groupsOff, KindFriendRequest, at("12:00"), true},
		{"silencio que cruza la medianoche, de noche", withQuiet(night), KindPlanInvite, at("23:30"), false},
		{"silencio que cruza la medianoche, de madrugada", withQuiet(night), KindPlanInvite, at("06:59"), false},
		{"fin del silencio excluido", withQuiet(night), KindPlanInvite, at("07:00"), true},
		{"silencio de tarde, dentro", withQuiet(afternoon), KindFriendAccepted, at("14:00"), false},
		{"silencio de tarde, fuera", withQuiet(afternoon), KindFriendAccepted, at("16:01"), true},
		{"horario mal formado no silencia", withQuiet(&usuarios.QuietHours{From: "25:00", To: "07:00"}), KindPlanInvite, at("23:30"), true},
		{"la hora se lee en México", withQuiet(night), KindPlanInvite, time.Date(2026, 11, 13, 5, 0, 0, 0, time.UTC), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Wanted(c.prefs, c.kind, c.now))
		})
	}
}

func TestPushText_GenericoParaCadaTipo(t *testing.T) {
	for _, kind := range []string{KindFriendRequest, KindFriendAccepted, KindPlanInvite, KindVoteOpened, KindVoteClosed} {
		assert.NotEmpty(t, pushText[kind], kind)
	}
}
