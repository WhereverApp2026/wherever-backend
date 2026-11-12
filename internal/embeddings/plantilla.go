// Package embeddings guarda el vector de significado de cada negocio aprobado (spec nv-50):
// la plantilla del texto público que se vectoriza, la colección `vectores_negocios` y el
// recálculo idempotente. El proveedor queda detrás de ia.Vectorizador.
package embeddings

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// VersionPlantilla cambia cuando cambia el texto que se arma: todos los vectores quedan
// desactualizados y el siguiente recálculo los rehace.
const VersionPlantilla = 1

// Fuente son los únicos campos del negocio que se vectorizan. Todos son públicos (los muestra
// la ficha); dueño, teléfono, dirección, ubicación y QR nunca entran.
type Fuente struct {
	ID          bson.ObjectID `bson:"_id"`
	Nombre      string        `bson:"nombre"`
	Categoria   string        `bson:"categoria"`
	Etiquetas   []string      `bson:"etiquetas"`
	Descripcion string        `bson:"descripcion"`
}

// DesdeNegocio toma solo los campos públicos de la plantilla.
func DesdeNegocio(b negocios.Business) Fuente {
	return Fuente{ID: b.ID, Nombre: b.Name, Categoria: b.Category, Etiquetas: b.Tags, Descripcion: b.Description}
}

// Documento arma el texto: el nombre es el título y el cuerpo lleva el giro, las etiquetas y la
// descripción. Pasa por ia.Anonimizar: una descripción con teléfono o correo no los manda.
func Documento(f Fuente) ia.Documento {
	partes := make([]string, 0, 3)
	if giro := nombreGiro(f.Categoria); giro != "" {
		partes = append(partes, giro+".")
	}
	if len(f.Etiquetas) > 0 {
		partes = append(partes, "Etiquetas: "+strings.Join(f.Etiquetas, ", ")+".")
	}
	if d := strings.TrimSpace(f.Descripcion); d != "" {
		partes = append(partes, d)
	}
	return ia.Documento{
		Titulo: ia.Anonimizar(strings.TrimSpace(f.Nombre)),
		Texto:  ia.Anonimizar(strings.Join(partes, " ")),
	}
}

// Version identifica plantilla, modelo y texto. Si cualquiera cambia, el vector se rehace.
func Version(modelo string, d ia.Documento) string {
	suma := sha256.Sum256([]byte(d.Titulo + "\x00" + d.Texto))
	return "v" + strconv.Itoa(VersionPlantilla) + ":" + modelo + ":" + hex.EncodeToString(suma[:8])
}

func nombreGiro(id string) string {
	for _, c := range negocios.CategoryCatalog() {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}
