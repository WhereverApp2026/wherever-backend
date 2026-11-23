package notificaciones

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestCategoryOf_TipoACategoria(t *testing.T) {
	cases := map[string]string{
		KindLike: CategorySocial, KindComment: CategorySocial, KindReviewUseful: CategorySocial,
		KindFriendRequest: CategorySocial, KindPlanInvite: CategorySocial, KindGroupMessage: CategorySocial,
		KindMomentTagged: CategorySocial, KindMomentThanked: CategorySocial, KindMomentFeatured: CategorySocial,
		KindReservaMensaje: CategoryBookings, KindReservaRecordatorio: CategoryBookings,
		KindVoteOpened: CategoryPlans, KindVoteClosed: CategoryPlans, KindTimeToLeave: CategoryPlans,
		KindBusinessReviewed: "", KindReviewPending: "", KindCouponExpiring: "",
	}
	for kind, want := range cases {
		assert.Equal(t, want, CategoryOf(kind), kind)
	}
}

func TestKindsOf_CubreTodosLosTiposDeLaCategoria(t *testing.T) {
	total := 0
	for _, c := range Categories() {
		kinds := KindsOf(c)
		assert.NotEmpty(t, kinds, c)
		for _, k := range kinds {
			assert.Equal(t, c, CategoryOf(k))
		}
		total += len(kinds)
	}
	assert.Len(t, categoryOf, total, "cada tipo con categoría sale en una sola pestaña")
	assert.Empty(t, KindsOf("otra"))
	for kind := range pushText {
		assert.Contains(t, allKinds, kind, "todo tipo con texto de push está en allKinds")
	}
}

func TestNewUnread_SumaPorCategoria(t *testing.T) {
	got := newUnread(map[string]int64{KindLike: 2, KindComment: 1, KindReservaRecordatorio: 3, KindBusinessReviewed: 4})
	assert.Equal(t, int64(10), got.Total)
	assert.Equal(t, map[string]int64{CategorySocial: 3, CategoryBookings: 3, CategoryPlans: 0}, got.ByCategory)
	empty := newUnread(map[string]int64{})
	assert.Equal(t, int64(0), empty.Total)
	assert.Len(t, empty.ByCategory, 3, "las tres pestañas siempre salen")
}

func TestStorable_InterruptoresYHorarioSilencioso(t *testing.T) {
	off := false
	on := usuarios.DefaultPreferences()
	socialOff := usuarios.DefaultPreferences()
	socialOff.Notifications.Social = &off
	remindersOff := usuarios.DefaultPreferences()
	remindersOff.Notifications.PlanReminders = &off
	couponsOff := usuarios.DefaultPreferences()
	couponsOff.Notifications.Coupons = false
	reviewsOff := usuarios.DefaultPreferences()
	reviewsOff.Notifications.Reviews = false
	quiet := usuarios.DefaultPreferences()
	quiet.QuietHours = &usuarios.QuietHours{From: "22:00", To: "07:00"}

	cases := []struct {
		name  string
		prefs usuarios.Preferences
		kind  string
		hhmm  string
		want  bool
	}{
		{"social ausente = encendido", on, KindLike, "12:00", true},
		{"social apagado: me gusta", socialOff, KindLike, "12:00", false},
		{"social apagado: comentario", socialOff, KindComment, "12:00", false},
		{"social apagado: útil", socialOff, KindReviewUseful, "12:00", false},
		{"social apagado no toca amistades", socialOff, KindFriendRequest, "12:00", true},
		{"cupones apagado", couponsOff, KindCouponExpiring, "12:00", false},
		{"reseñas apagado", reviewsOff, KindReviewPending, "12:00", false},
		{"recordatorios del plan apagado", remindersOff, KindTimeToLeave, "12:00", false},
		{"hora de salir en horario silencioso", quiet, KindTimeToLeave, "23:00", false},
		{"hora de salir fuera del silencio", quiet, KindTimeToLeave, "12:00", true},
		{"un me gusta en silencio se guarda (el push espera)", quiet, KindLike, "23:00", true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, Storable(tc.prefs, tc.kind, at(tc.hhmm)), tc.name)
	}
	assert.False(t, Wanted(socialOff, KindLike, at("12:00")), "apagado tampoco suena")
	assert.True(t, HasSwitch(KindTimeToLeave))
	assert.False(t, HasSwitch(KindReservaRecordatorio), "reservas no se apagan")
}

func TestChannelOf_AvisosNuevos(t *testing.T) {
	assert.Equal(t, ChannelCoupons, ChannelOf(KindCouponExpiring))
	assert.Equal(t, ChannelReviews, ChannelOf(KindReviewPending))
	assert.Equal(t, ChannelGroups, ChannelOf(KindTimeToLeave))
}
