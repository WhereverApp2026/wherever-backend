package notificaciones

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Categorías de la bandeja (nv-86). Un tipo sin categoría solo sale en «todo».
const (
	CategorySocial   = "social"
	CategoryBookings = "reservas"
	CategoryPlans    = "planes"
)

// Categories son las pestañas de la bandeja, en orden.
func Categories() []string { return []string{CategorySocial, CategoryBookings, CategoryPlans} }

var categoryOf = map[string]string{
	KindFriendRequest:  CategorySocial,
	KindFriendAccepted: CategorySocial,
	KindPlanInvite:     CategorySocial,
	KindGroupMessage:   CategorySocial,
	KindMomentTagged:   CategorySocial,
	KindMomentThanked:  CategorySocial,
	KindMomentFeatured: CategorySocial,
	KindLike:           CategorySocial,
	KindComment:        CategorySocial,
	KindReviewUseful:   CategorySocial,
	KindDirectMessage:  CategorySocial,

	KindReservaMensaje:      CategoryBookings,
	KindReservaRecordatorio: CategoryBookings,

	KindVoteOpened:  CategoryPlans,
	KindVoteClosed:  CategoryPlans,
	KindTimeToLeave: CategoryPlans,
}

// CategoryOf es la pestaña del tipo; vacío = solo en «todo».
func CategoryOf(kind string) string { return categoryOf[kind] }

// KindsOf son los tipos de una categoría, ordenados para que la consulta sea estable.
func KindsOf(category string) []string {
	out := []string{}
	for _, k := range allKinds {
		if categoryOf[k] == category {
			out = append(out, k)
		}
	}
	return out
}

// allKinds son todos los tipos conocidos, en un orden fijo.
var allKinds = []string{
	KindFriendRequest, KindFriendAccepted, KindPlanInvite, KindVoteOpened, KindVoteClosed,
	KindBusinessReviewed, KindGroupMessage, KindMomentTagged, KindMomentThanked, KindMomentFeatured,
	KindReservaMensaje, KindReservaRecordatorio, KindLike, KindComment, KindReviewUseful,
	KindReviewPending, KindCouponExpiring, KindTimeToLeave, KindDirectMessage,
}

// switchOn dice si el interruptor de `preferencias.notificaciones` que apaga el tipo del todo: con
// él apagado el aviso no se guarda ni se envía (nv-86). Los tipos sin interruptor siempre se
// guardan; `grupos` solo calla el push (nv-41).
func switchOn(prefs usuarios.Notifications, kind string) (bool, bool) {
	switch kind {
	case KindLike, KindComment, KindReviewUseful, KindMomentTagged, KindMomentThanked, KindMomentFeatured:
		return prefs.SocialOn(), true
	case KindCouponExpiring:
		return prefs.Coupons, true
	case KindReviewPending:
		return prefs.Reviews, true
	case KindTimeToLeave:
		return prefs.PlanRemindersOn(), true
	}
	return true, false
}

// HasSwitch dice si el tipo depende de un interruptor; solo entonces Emit lee a la persona.
func HasSwitch(kind string) bool {
	_, ok := switchOn(usuarios.Notifications{}, kind)
	return ok
}

// Storable dice si el aviso se guarda: el interruptor del tipo encendido y, para la hora de
// salir, fuera del horario silencioso (pasado ese momento ya no sirve).
func Storable(prefs usuarios.Preferences, kind string, now time.Time) bool {
	if on, _ := switchOn(prefs.Notifications, kind); !on {
		return false
	}
	if kind == KindTimeToLeave {
		return !inQuietHours(prefs.QuietHours, now.In(negocios.MexicoCity()))
	}
	return true
}
