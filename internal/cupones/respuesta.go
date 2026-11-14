package cupones

import "time"

func valorPublico(tipo string, v int64) *int64 {
	if tipo != TipoPorcentaje && tipo != TipoMonto {
		return nil
	}
	return &v
}

// publicoResponse es el cupón sin contadores ni versión.
func publicoResponse(c Cupon) CuponResponse {
	dias := c.DiasValidos
	if dias == nil {
		dias = []int{}
	}
	return CuponResponse{
		ID: c.ID.Hex(), NegocioID: c.NegocioID.Hex(), NegocioNombre: c.NegocioNombre, Titulo: c.Titulo, Tipo: c.Tipo,
		Valor: valorPublico(c.Tipo, c.Valor), Condiciones: c.Condiciones, VigenciaInicio: c.VigenciaInicio.UTC(), VigenciaFin: c.VigenciaFin.UTC(),
		Relampago: c.Relampago, DiasValidos: dias, HorasValidas: c.HorasValidas, Categoria: c.Categoria, Estado: c.Estado,
	}
}

// duenoResponse agrega límite, usos, vistas, guardados y versión.
func duenoResponse(c Cupon) CuponResponse {
	out := publicoResponse(c)
	out.LimiteUsos, out.UsosActuales, out.Vistas, out.Guardados, out.Version = &c.LimiteUsos, &c.UsosActuales, &c.Vistas, &c.Guardados, &c.Version
	return out
}

// carteraResponse es el cupón guardado; `segundos_restantes` solo en «por vencer».
func carteraResponse(c Cartera, now time.Time, conRestante bool) CarteraResponse {
	out := CarteraResponse{
		ID: c.ID.Hex(), CuponID: c.CuponID.Hex(), NegocioID: c.NegocioID.Hex(), Titulo: c.Resumen.Titulo, Tipo: c.Resumen.Tipo,
		Valor: valorPublico(c.Resumen.Tipo, c.Resumen.Valor), NegocioNombre: c.Resumen.NegocioNombre, Estado: c.Estado,
		VenceEn: c.VenceEn.UTC(), CreadoEn: c.CreadoEn.UTC(),
	}
	if c.CanjeadoEn != nil {
		t := c.CanjeadoEn.UTC()
		out.CanjeadoEn = &t
	}
	if conRestante {
		s := int64(c.VenceEn.Sub(now) / time.Second)
		out.SegundosRestantes = &s
	}
	return out
}

// canjeResponse es el código vivo y el texto del QR que dibuja la app.
func canjeResponse(c Cartera) CanjeResponse {
	out := CanjeResponse{Codigo: c.CodigoCanje, QR: "wherever://canje/" + c.CodigoCanje}
	if c.CanjeExpiraEn != nil {
		out.ExpiraEn = c.CanjeExpiraEn.UTC()
	}
	return out
}
