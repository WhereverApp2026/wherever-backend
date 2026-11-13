// Package votaciones deja que el grupo de un plan elija entre 2 o 3 negocios para una parada
// (nv-39, US-099): abrir, votar, cerrar a mano o al vencer, y aplicar la ganadora al plan.
package votaciones

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection guarda las votaciones (J3).
const Collection = "votaciones"

// Estados de una votación. Votar solo se puede mientras está abierta y antes de `cierra_en`.
const (
	StatusOpen   = "abierta"
	StatusClosed = "cerrada"
)

// Quién la cerró.
const (
	ClosedByOrganizer = "organizador"
	ClosedByDeadline  = "vencimiento"
)

// Límites de la spec: 2 o 3 opciones y hora límite de 5 minutos a 7 días.
const (
	MinOptions  = 2
	MaxOptions  = 3
	MinDuration = 5 * time.Minute
	MaxDuration = 7 * 24 * time.Hour
)

// Vote es un documento de `votaciones`. `votos` va de id de usuario (hex) a negocio elegido:
// un voto por persona que se reemplaza mientras está abierta. `ganadora` y `estado` cambian
// juntos en una sola operación, condicionada a `votos_version`; `aplicada` dice si el plan
// ya cambió (falta = pendiente).
type Vote struct {
	ID      bson.ObjectID            `bson:"_id"`
	GroupID bson.ObjectID            `bson:"grupo_id"`
	PlanID  bson.ObjectID            `bson:"plan_id"`
	StopID  bson.ObjectID            `bson:"parada_id"`
	Options []bson.ObjectID          `bson:"opciones"`
	Ballots map[string]bson.ObjectID `bson:"votos"`
	// BallotsVersion sube con cada voto: cerrar exige la misma que se contó.
	BallotsVersion int            `bson:"votos_version"`
	ClosesAt       time.Time      `bson:"cierra_en"`
	Status         string         `bson:"estado"`
	Winner         *bson.ObjectID `bson:"ganadora,omitempty"`
	ClosedBy       string         `bson:"cerrada_por,omitempty"`
	ClosedAt       *time.Time     `bson:"cerrada_en,omitempty"`
	Applied        *bool          `bson:"aplicada,omitempty"`
	// ApplyingUntil reserva la aplicación para una instancia durante un minuto.
	ApplyingUntil *time.Time    `bson:"aplicando_hasta,omitempty"`
	Reason        string        `bson:"motivo,omitempty"`
	CreatedBy     bson.ObjectID `bson:"creada_por"`
	CreatedAt     time.Time     `bson:"creada_en"`
	UpdatedAt     time.Time     `bson:"actualizada_en"`
}

// Counts cuenta los votos de cada opción, en el orden de `opciones`. Un voto por una opción
// que no existe no cuenta.
func (v Vote) Counts() []int {
	out := make([]int, len(v.Options))
	for _, choice := range v.Ballots {
		for i, opt := range v.Options {
			if opt == choice {
				out[i]++
			}
		}
	}
	return out
}

// WinnerOf es la opción con más votos; en empate (también sin votos) gana la propuesta
// primero (US-099).
func WinnerOf(v Vote) bson.ObjectID {
	counts := v.Counts()
	best := 0
	for i := range counts {
		if counts[i] > counts[best] {
			best = i
		}
	}
	return v.Options[best]
}
