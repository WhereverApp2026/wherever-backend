package panel

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Cupo por hora y día (nv-88, `55_NegHoras`): el dueño ve y reemplaza la tabla de lugares de
// cada hora abierta; la disponibilidad de reservas la usa.

const campoCupo = "cupo_por_hora"

// Cupo da la tabla de las horas dentro de `horarios`, con sus lugares y su afluencia.
func (s *Service) Cupo(ctx context.Context, claims *middleware.Claims, id string) (CupoResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return CupoResponse{}, err
	}
	return tablaCupo(b), nil
}

// CambiarCupo reemplaza la tabla completa con `If-Match`. Pide «Horas y cupo» guardado y un
// negocio publicado; horas fuera del horario, repetidas o con más de `max_personas` = 422.
func (s *Service) CambiarCupo(ctx context.Context, claims *middleware.Claims, id string, version int, req CupoRequest) (CupoResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return CupoResponse{}, err
	}
	if !slices.Contains(publicados, b.State) {
		return CupoResponse{}, errEstado()
	}
	if b.ReservasConfig == nil {
		return CupoResponse{}, invalido(campoCupo, "primero guarda «Horas y cupo» (reservas-config)")
	}
	cupos, err := validarCupo(req.CupoPorHora, b.Schedules, b.ReservasConfig.MaxPersonas)
	if err != nil {
		return CupoResponse{}, err
	}
	nb, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "version", Value: version},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: publicados}}}, noEliminado()},
		bson.D{{Key: "reservas_config." + campoCupo, Value: cupos}})
	if errors.Is(err, negocios.ErrNoAplica) {
		return CupoResponse{}, apperr.Conflict("el negocio cambió; vuelve a cargarlo")
	}
	if err != nil {
		return CupoResponse{}, errStore()
	}
	s.despues(ctx, nb.ID, "")
	return tablaCupo(nb), nil
}

// validarCupo exige horas "HH:00" dentro del horario, sin repetir y sin pasar de `maxPersonas`
// (0 = sin tope). Devuelve la lista ordenada por día y hora.
func validarCupo(req []CupoHoraRequest, horarios []negocios.Horario, maxPersonas int) ([]negocios.CupoHora, error) {
	abiertas := negocios.HorasAbiertas(horarios)
	var vistas [7][24]bool
	out := make([]negocios.CupoHora, 0, len(req))
	for i, e := range req {
		campo := campoCupo + "[" + strconv.Itoa(i) + "]"
		h, ok := negocios.HoraEntera(e.Hora)
		if !ok {
			return nil, invalido(campo+".hora", "usa una hora entera HH:00")
		}
		d := *e.Dia
		if !abiertas[d][h] {
			return nil, invalido(campo+".hora", "fuera del horario del negocio")
		}
		if vistas[d][h] {
			return nil, invalido(campo+".hora", "hora repetida")
		}
		vistas[d][h] = true
		if maxPersonas > 0 && *e.Lugares > maxPersonas {
			return nil, invalido(campo+".lugares", "no puede superar max_personas")
		}
		out = append(out, negocios.CupoHora{Dia: d, Hora: negocios.FormatoHora(h), Lugares: *e.Lugares})
	}
	slices.SortFunc(out, func(a, b negocios.CupoHora) int {
		if a.Dia != b.Dia {
			return a.Dia - b.Dia
		}
		return cmp.Compare(a.Hora, b.Hora)
	})
	return out, nil
}

// tablaCupo arma los 7 días con sus horas abiertas; sin «Horas y cupo» todo va en 0.
func tablaCupo(b negocios.Business) CupoResponse {
	cfg := negocios.ReservasConfig{}
	if b.ReservasConfig != nil {
		cfg = *b.ReservasConfig
	}
	abiertas := negocios.HorasAbiertas(b.Schedules)
	out := CupoResponse{Dias: make([]CupoDiaResponse, 0, len(abiertas)), CapacidadPorHorario: cfg.CapacidadPorHorario, Version: b.Version}
	for d, horas := range abiertas {
		dia := CupoDiaResponse{Dia: d, Horas: []CupoHoraResponse{}}
		for h, abierta := range horas {
			if !abierta {
				continue
			}
			nivel, _ := negocios.CrowdAt(b.Traffic, d, h)
			dia.Horas = append(dia.Horas, CupoHoraResponse{Hora: negocios.FormatoHora(h), Lugares: cfg.CupoEn(d, h), Afluencia: nivel})
		}
		out.Dias = append(out.Dias, dia)
	}
	return out
}
