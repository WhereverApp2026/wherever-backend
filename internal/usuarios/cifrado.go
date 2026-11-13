package usuarios

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
)

// Campos cifrados de `usuarios` (nv-49). El resto del código ve el valor en claro: el
// repositorio cifra al escribir y descifra al leer.
const (
	fieldAllergies = "gustos.alergias"
	fieldBirthDate = "fecha_nacimiento"
	birthLayout    = "2006-01-02"
)

// ErrCipher: un valor cifrado no abrió (llave faltante o dato alterado). Falla cerrado.
var ErrCipher = errors.New("usuarios: no se pudo descifrar un dato sensible")

// WithCipher activa el cifrado de alergias y fecha de nacimiento. Sin llavero (fuera de
// prod sin DATA_KEYS) se guardan en claro.
func (r *Repository) WithCipher(l *cifrado.Llavero) *Repository {
	r.cipher = l
	return r
}

func place(id bson.ObjectID, field string) cifrado.Lugar {
	return cifrado.Lugar{Coleccion: Collection, Campo: field, ID: id.Hex()}
}

func (r *Repository) seal(id bson.ObjectID, field, plain string) (string, error) {
	if r.cipher == nil || plain == "" {
		return plain, nil
	}
	return r.cipher.Cifrar(place(id, field), plain)
}

func (r *Repository) unseal(id bson.ObjectID, field, stored string) (string, error) {
	if !cifrado.EsCifrado(stored) {
		return stored, nil // dato en claro de antes de la migración
	}
	if r.cipher == nil {
		return "", ErrCipher
	}
	plain, err := r.cipher.Descifrar(place(id, field), stored)
	if err != nil {
		return "", ErrCipher
	}
	return plain, nil
}

// sealedBirth guarda la fecha como texto cifrado (o la fecha en claro sin llavero).
func (r *Repository) sealedBirth(id bson.ObjectID, birth *time.Time) (any, error) {
	if birth == nil {
		return nil, nil
	}
	if r.cipher == nil {
		return *birth, nil
	}
	return r.seal(id, fieldBirthDate, birth.UTC().Format(birthLayout))
}

// sealUser devuelve la copia que va a la base, con los campos sensibles cifrados. No toca el
// usuario de quien llama.
func (r *Repository) sealUser(u *User) (*User, error) {
	doc := *u
	stored, err := r.sealedBirth(u.ID, u.BirthDate)
	if err != nil {
		return nil, err
	}
	doc.BirthDateStored = stored
	if u.BirthDate != nil {
		doc.Adult = true // la edad ya se validó al registrar (nv-05); se guarda el booleano
	}
	if u.Tastes != nil {
		tastes := *u.Tastes
		if tastes.Allergies, err = r.seal(u.ID, fieldAllergies, tastes.Allergies); err != nil {
			return nil, err
		}
		doc.Tastes = &tastes
	}
	return &doc, nil
}

// open descifra lo que se leyó de la base.
func (r *Repository) open(u *User) error {
	if u.Tastes != nil && u.Tastes.Allergies != "" {
		plain, err := r.unseal(u.ID, fieldAllergies, u.Tastes.Allergies)
		if err != nil {
			return err
		}
		u.Tastes.Allergies = plain
	}
	switch v := u.BirthDateStored.(type) {
	case bson.DateTime:
		t := v.Time().UTC()
		u.BirthDate = &t
	case string:
		plain, err := r.unseal(u.ID, fieldBirthDate, v)
		if err != nil {
			return err
		}
		t, err := time.Parse(birthLayout, plain)
		if err != nil {
			return ErrCipher
		}
		u.BirthDate = &t
	}
	u.BirthDateStored = nil
	return nil
}

// sealSet cifra los campos sensibles de un `$set` antes de mandarlo.
func (r *Repository) sealSet(id bson.ObjectID, set bson.D) (bson.D, error) {
	out := make(bson.D, 0, len(set))
	for _, e := range set {
		v, err := r.sealValue(id, e.Key, e.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, bson.E{Key: e.Key, Value: v})
	}
	return out, nil
}

func (r *Repository) sealValue(id bson.ObjectID, key string, value any) (any, error) {
	switch key {
	case "gustos":
		switch t := value.(type) {
		case Tastes:
			copied := t
			var err error
			copied.Allergies, err = r.seal(id, fieldAllergies, t.Allergies)
			return copied, err
		case *Tastes:
			if t == nil {
				return value, nil
			}
			copied := *t
			var err error
			copied.Allergies, err = r.seal(id, fieldAllergies, t.Allergies)
			return &copied, err
		}
	case fieldAllergies:
		if s, ok := value.(string); ok {
			return r.seal(id, fieldAllergies, s)
		}
	case fieldBirthDate:
		switch t := value.(type) {
		case time.Time:
			return r.sealedBirth(id, &t)
		case *time.Time:
			return r.sealedBirth(id, t)
		}
	}
	return value, nil
}

func idOf(filter bson.D) bson.ObjectID {
	for _, e := range filter {
		if id, ok := e.Value.(bson.ObjectID); ok && e.Key == "_id" {
			return id
		}
	}
	return bson.ObjectID{}
}

// MigrationStats cuenta lo que hace la migración.
type MigrationStats struct {
	Revisados  int
	Pendientes int
	Cifrados   int
}

// Reseal cifra (o re-cifra con la llave activa) las alergias y la fecha de nacimiento de
// todas las cuentas. Es idempotente: lo ya cifrado con la llave activa no se toca. Con
// apply=false solo cuenta. No sube `version`: no es una edición de la persona.
func (r *Repository) Reseal(ctx context.Context, apply bool) (MigrationStats, error) {
	var stats MigrationStats
	if r.cipher == nil {
		return stats, errors.New("usuarios: sin DATA_KEYS no hay con qué cifrar")
	}
	cur, err := r.col.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "gustos.alergias", Value: 1}, {Key: fieldBirthDate, Value: 1}}))
	if err != nil {
		return stats, err
	}
	defer func() { _ = cur.Close(ctx) }()
	for cur.Next(ctx) {
		var u User
		if err := cur.Decode(&u); err != nil {
			return stats, err
		}
		stats.Revisados++
		if !r.needsReseal(u) {
			continue
		}
		stats.Pendientes++
		if !apply {
			continue
		}
		if err := r.resealOne(ctx, u); err != nil {
			return stats, err
		}
		stats.Cifrados++
	}
	return stats, cur.Err()
}

func (r *Repository) needsReseal(u User) bool {
	stale := func(v string) bool {
		if v == "" {
			return false
		}
		if !cifrado.EsCifrado(v) {
			return true
		}
		id, err := cifrado.LlaveDe(v)
		return err != nil || id != r.cipher.Activa()
	}
	if u.Tastes != nil && stale(u.Tastes.Allergies) {
		return true
	}
	switch v := u.BirthDateStored.(type) {
	case bson.DateTime:
		return true
	case string:
		return stale(v)
	}
	return false
}

func (r *Repository) resealOne(ctx context.Context, u User) error {
	if err := r.open(&u); err != nil {
		return err
	}
	set := bson.D{}
	if u.Tastes != nil && u.Tastes.Allergies != "" {
		v, err := r.seal(u.ID, fieldAllergies, u.Tastes.Allergies)
		if err != nil {
			return err
		}
		set = append(set, bson.E{Key: fieldAllergies, Value: v})
	}
	if u.BirthDate != nil {
		v, err := r.sealedBirth(u.ID, u.BirthDate)
		if err != nil {
			return err
		}
		set = append(set, bson.E{Key: fieldBirthDate, Value: v}, bson.E{Key: "mayor_de_edad", Value: true})
	}
	if len(set) == 0 {
		return nil
	}
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: u.ID}}, bson.D{{Key: "$set", Value: set}})
	return err
}
