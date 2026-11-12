package sugerencias

import "github.com/WhereverApp2026/wherever-backend/internal/personas"

// Suggestion es una persona que quizá conozco y cuántos amigos tenemos en común.
type Suggestion struct {
	Person personas.Person `json:"persona"`
	Common int64           `json:"amigos_en_comun"`
}

// SuggestionsResponse es la respuesta de GET /amistades/sugerencias. Degraded indica que
// Neo4j no respondió: la lista va vacía y no es un error.
type SuggestionsResponse struct {
	Results  []Suggestion `json:"resultados"`
	Degraded bool         `json:"degradado"`
}

// FriendsAtResponse es la respuesta de GET /lugares/{id}/amigos: cuántos amigos fueron y
// hasta 3 de ellos.
type FriendsAtResponse struct {
	Total    int               `json:"total"`
	Friends  []personas.Person `json:"amigos"`
	Degraded bool              `json:"degradado"`
}
