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
		out.Results = append(out.Results, Person{ID: u.ID.Hex(), Name: u.Name, Username: u.Username, PhotoURL: u.PhotoURL})
	}
	return out
}
