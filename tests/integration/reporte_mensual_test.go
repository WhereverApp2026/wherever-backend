package integration

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/estadisticastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func TestReporteMensual_PDFConTotales(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	b, token := dueno(t, env, "duena-reporte@wherever.test", reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: b.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "creado_en", Value: time.Now().AddDate(-2, 0, 0)}}}})
	require.NoError(t, err)

	hoy := time.Now().In(zonaMX(t))
	mes := time.Date(hoy.Year(), hoy.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	for _, dia := range []int{2, 9, 20} {
		estadisticastest.Sembrar(t, env.Mongo.DB(), b.ID, mes.AddDate(0, 0, dia-1).Format(estadisticas.FormatoDia), estadisticas.Diaria{VistasFicha: 41, Reservas: 3})
	}
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/reportes/mensual?mes="+mes.Format(estadisticas.FormatoMes), nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="wherever-`+mes.Format(estadisticas.FormatoMes)+`.pdf"`, resp.Header.Get("Content-Disposition"))
	archivo, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(archivo) > 5 && string(archivo[:5]) == "%PDF-")
	texto := estadisticastest.Texto(archivo)
	assert.Contains(t, texto, b.Name)
	assert.Contains(t, texto, estadisticas.NombreMes(mes))
	assert.Contains(t, texto, "Visitas a la ficha\n123\n", "41 × 3 días")
	assert.Contains(t, texto, "Sin canjes este mes.")
}

func TestReporteMensual_MesFueraDeRango(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, token := dueno(t, env, "duena-reporte-mes@wherever.test", reservastest.Config{})
	ruta := "/api/v1/negocios/" + b.ID.Hex() + "/reportes/mensual?mes="
	hoy := time.Now().In(zonaMX(t))
	actual := time.Date(hoy.Year(), hoy.Month(), 1, 0, 0, 0, 0, time.UTC)
	requireStatus(t, sendJSON(t, app, http.MethodGet, ruta+actual.AddDate(0, 1, 0).Format(estadisticas.FormatoMes), nil, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, ruta+actual.AddDate(0, -13, 0).Format(estadisticas.FormatoMes), nil, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, ruta+"2026-13", nil, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, ruta[:len(ruta)-len("?mes=")], nil, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, ruta+actual.Format(estadisticas.FormatoMes), nil, token), http.StatusOK)
}
