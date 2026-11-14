// Package reservastest siembra negocios listos para reservar (nv-59) en la base real de las
// pruebas: aprobados, abiertos todo el día y con «Horas y cupo».
package reservastest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Config son las opciones de «Horas y cupo» de la prueba.
type Config struct {
	Capacidad      int
	Manual         bool
	PermiteCambios bool
	Anticipo       negocios.Anticipo
	Politica       string
	SinReservas    bool
}

// Negocio inserta un negocio aprobado del dueño, abierto de 00:00 a 24:00 los siete días con
// turnos de 60 minutos.
func Negocio(t testing.TB, db *mongo.Database, owner bson.ObjectID, c Config) negocios.Business {
	t.Helper()
	if c.Capacidad == 0 {
		c.Capacidad = 10
	}
	if c.Politica == "" {
		c.Politica = "flexible"
	}
	confirmacion := "automatica"
	if c.Manual {
		confirmacion = "manual"
	}
	if c.Anticipo.Tipo == "" {
		c.Anticipo.Tipo = "fijo"
	}
	horarios := make([]negocios.Horario, 0, 7)
	for d := range 7 {
		horarios = append(horarios, negocios.Horario{Dia: d, Abre: "00:00", Cierra: "24:00"})
	}
	id := bson.NewObjectID()
	now := time.Now().UTC()
	b := negocios.Business{
		ID: id, OwnerID: owner, Slug: "reservas-" + id.Hex(), Name: "Fonda " + id.Hex()[18:], NormalizedName: "fonda " + id.Hex()[18:],
		Category: "restaurante", Tags: []string{}, PriceLevel: 2, AveragePrice: 200,
		Location:  negocios.Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}},
		Schedules: horarios, ScheduleMin: []negocios.VentanaMin{}, Photos: []negocios.Foto{}, Services: []string{}, DietOptions: []string{}, Allergens: []string{},
		AcceptsReservations: !c.SinReservas, State: negocios.StatusApproved, Plan: negocios.PlanFree, QRCheckin: "qr-" + id.Hex(),
		Step: negocios.StepComplete, Version: 1, CreatedAt: now, UpdatedAt: now,
		ReservasConfig: &negocios.ReservasConfig{
			AceptaReservas: !c.SinReservas, CapacidadPorHorario: c.Capacidad, DuracionTurnoMin: 60, Anticipo: c.Anticipo,
			PoliticaCancelacion: c.Politica, Confirmacion: confirmacion, PermiteCambios: c.PermiteCambios,
		},
	}
	_, err := db.Collection(negocios.Collection).InsertOne(context.Background(), b)
	require.NoError(t, err)
	return b
}

// Manana es mañana a esa hora de México, en UTC (siempre un turno futuro de Negocio).
func Manana(hora int) time.Time {
	y, m, d := time.Now().In(negocios.MexicoCity()).AddDate(0, 0, 1).Date()
	return time.Date(y, m, d, hora, 0, 0, 0, negocios.MexicoCity()).UTC()
}

// FechaManana es mañana como "AAAA-MM-DD" de México.
func FechaManana() string {
	return time.Now().In(negocios.MexicoCity()).AddDate(0, 0, 1).Format("2006-01-02")
}
