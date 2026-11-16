// Package estadisticastest ayuda a probar nv-74: siembra días y lee el texto de los PDF.
package estadisticastest

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
)

// Dia es el día local de t en America/Mexico_City (`AAAA-MM-DD`).
func Dia(t time.Time) string {
	zona, err := time.LoadLocation(estadisticas.Zona)
	if err != nil {
		zona = time.UTC
	}
	return t.In(zona).Format(estadisticas.FormatoDia)
}

// Ayer es el día local de ayer.
func Ayer() string { return Dia(time.Now().AddDate(0, 0, -1)) }

// Sembrar guarda el día del negocio tal cual (sin pasar por el trabajo).
func Sembrar(t testing.TB, db *mongo.Database, negocio bson.ObjectID, fecha string, d estadisticas.Diaria) {
	t.Helper()
	d.ID, d.NegocioID, d.Fecha = bson.NewObjectID(), negocio, fecha
	if d.PorHora == nil {
		d.PorHora = make([]int, 24)
	}
	if d.ActualizadoEn.IsZero() {
		d.ActualizadoEn = time.Now().UTC()
	}
	_, err := db.Collection(estadisticas.Collection).InsertOne(context.Background(), d)
	require.NoError(t, err)
}

// Leer trae el día del negocio; falla si no existe.
func Leer(t testing.TB, db *mongo.Database, negocio bson.ObjectID, fecha string) estadisticas.Diaria {
	t.Helper()
	var d estadisticas.Diaria
	require.NoError(t, db.Collection(estadisticas.Collection).FindOne(context.Background(),
		bson.D{{Key: "negocio_id", Value: negocio}, {Key: "fecha", Value: fecha}}).Decode(&d))
	return d
}

var cadena = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\) Tj`)

// desdeWinAnsi son los caracteres de WinAnsi fuera de Latin-1 que escribe internal/estadisticas/pdf.
var desdeWinAnsi = map[byte]rune{0x80: '€', 0x85: '…', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—'}

// Texto junta, una por línea, las cadenas que el PDF dibuja con `Tj`, ya decodificadas.
func Texto(archivo []byte) string {
	var sb strings.Builder
	for _, m := range cadena.FindAllSubmatch(archivo, -1) {
		raw := m[1]
		for i := 0; i < len(raw); i++ {
			c := raw[i]
			if c == '\\' && i+1 < len(raw) {
				i++
				c = raw[i]
			}
			if r, ok := desdeWinAnsi[c]; ok {
				sb.WriteRune(r)
				continue
			}
			sb.WriteRune(rune(c))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
