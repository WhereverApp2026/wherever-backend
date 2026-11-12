package clima

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// OpenMeteoURL es la API pública de Open-Meteo; no usa llave.
const OpenMeteoURL = "https://api.open-meteo.com"

// Plazo es el tiempo máximo de una consulta al proveedor (spec nv-25).
const Plazo = 3 * time.Second

// limiteCuerpo acota lo que se lee de la respuesta (la real pesa menos de 1 KB).
const limiteCuerpo = 64 << 10

// OpenMeteo consulta `GET /v1/forecast?current=…`.
type OpenMeteo struct {
	base string
	http *http.Client
}

// NewOpenMeteo recibe la URL base (OpenMeteoURL o un servidor de pruebas).
func NewOpenMeteo(base string) *OpenMeteo {
	return &OpenMeteo{base: base, http: &http.Client{Timeout: Plazo}}
}

type respuestaOpenMeteo struct {
	Current *struct {
		Time          string   `json:"time"`
		Temperature   *float64 `json:"temperature_2m"`
		WeatherCode   *int     `json:"weather_code"`
		IsDay         int      `json:"is_day"`
		Precipitation float64  `json:"precipitation"`
	} `json:"current"`
}

// Actual lee el clima actual. La hora viene en GMT.
func (o *OpenMeteo) Actual(ctx context.Context, lat, lng float64) (Lectura, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 2, 64))
	q.Set("longitude", strconv.FormatFloat(lng, 'f', 2, 64))
	q.Set("current", "temperature_2m,weather_code,is_day,precipitation")
	q.Set("timezone", "GMT")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base+"/v1/forecast?"+q.Encode(), nil)
	if err != nil {
		return Lectura{}, err
	}
	resp, err := o.http.Do(req)
	if err != nil {
		return Lectura{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Lectura{}, fmt.Errorf("%w: estado %d", ErrProveedor, resp.StatusCode)
	}
	var body respuestaOpenMeteo
	if err := json.NewDecoder(io.LimitReader(resp.Body, limiteCuerpo)).Decode(&body); err != nil {
		return Lectura{}, fmt.Errorf("%w: %w", ErrProveedor, err)
	}
	c := body.Current
	if c == nil || c.Temperature == nil || c.WeatherCode == nil {
		return Lectura{}, fmt.Errorf("%w: faltan campos", ErrProveedor)
	}
	observado, err := time.Parse("2006-01-02T15:04", c.Time)
	if err != nil {
		return Lectura{}, fmt.Errorf("%w: hora", ErrProveedor)
	}
	return Lectura{
		TemperaturaC: *c.Temperature, Codigo: *c.WeatherCode, EsDeDia: c.IsDay == 1,
		PrecipitacionMM: c.Precipitation, ObservadoEn: observado,
	}, nil
}
