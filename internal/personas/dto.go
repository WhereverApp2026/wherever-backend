package personas

import "github.com/WhereverApp2026/wherever-backend/internal/usuarios"

// Person es lo único que se muestra de otra persona: nunca correo, teléfono ni su hash.
type Person struct {
	ID       string `json:"id"`
	Name     string `json:"nombre"`
	Username string `json:"nombre_usuario"`
	PhotoURL string `json:"foto_url"`
}

// SearchResponse tiene la misma forma con o sin resultados.
type SearchResponse struct {
	Results []Person `json:"resultados"`
}

func newSearchResponse(users []usuarios.User) SearchResponse {
	out := SearchResponse{Results: make([]Person, 0, len(users))}
	for _, u := range users {
		out.Results = append(out.Results, newPerson(u))
	}
	return out
}

func newPerson(u usuarios.User) Person {
	return Person{ID: u.ID.Hex(), Name: u.Name, Username: u.Username, PhotoURL: u.PhotoURL}
}

// Relaciones de GET /personas/codigo/{codigo} (nv-64), vistas por quien pregunta.
const (
	RelationFriend   = "amigo"
	RelationSent     = "solicitud_enviada"
	RelationReceived = "solicitud_recibida"
	RelationNone     = "ninguna"
)

// Estados de `amistades` (nv-36). Se repiten aquí porque amistades importa este paquete.
const (
	friendshipPending  = "pendiente"
	friendshipAccepted = "aceptada"
)

// CodeResponse es el perfil público de quien tiene el código más la relación.
type CodeResponse struct {
	Person
	Relation string `json:"relacion"`
}

// relationOf traduce el estado de `amistades`; visible=false es un bloqueo (o un estado
// desconocido): se responde 404.
func relationOf(status string, mine bool) (rel string, visible bool) {
	switch status {
	case "":
		return RelationNone, true
	case friendshipAccepted:
		return RelationFriend, true
	case friendshipPending:
		if mine {
			return RelationSent, true
		}
		return RelationReceived, true
	default:
		return "", false
	}
}
