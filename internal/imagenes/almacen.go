package imagenes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// Errores del almacén.
var (
	// ErrSinObjeto: el original no está (nunca se subió o ya venció). Sin s3:ListBucket, S3
	// responde 403 en lugar de 404 a una clave inexistente; las dos caen aquí.
	ErrSinObjeto = errors.New("imagenes: el objeto no existe")
	// ErrObjetoGrande: el objeto pasa del límite (no debería: la política de subida lo impide).
	ErrObjetoGrande = errors.New("imagenes: el objeto pasa del límite")
)

// CacheInmutable es la cabecera de los tamaños publicados: el nombre nunca cambia (nv-44).
const CacheInmutable = "public, max-age=31536000, immutable"

// Subida es el formulario firmado: la app manda un POST multipart a URL con Campos y, al
// final, el archivo en el campo `file`.
type Subida struct {
	URL    string
	Campos map[string]string
}

// Almacen es el bucket de imágenes. Es una interfaz externa (S3): en pruebas va un doble.
type Almacen interface {
	// FirmarSubida firma un POST que solo acepta esa clave, ese tipo y hasta maxBytes.
	FirmarSubida(ctx context.Context, clave, tipo string, maxBytes int64, vence time.Duration) (Subida, error)
	// Leer devuelve el objeto completo si no pasa de max bytes.
	Leer(ctx context.Context, clave string, max int64) ([]byte, error)
	// Guardar escribe un tamaño publicado.
	Guardar(ctx context.Context, clave, tipo string, datos []byte) error
	// Borrar elimina las claves; una inexistente no es error.
	Borrar(ctx context.Context, claves ...string) error
}

// S3Config configura el adaptador. Endpoint y Credenciales solo se usan en pruebas; en AWS
// las credenciales salen del rol de la instancia (IMDSv2).
type S3Config struct {
	Bucket       string
	Region       string
	Endpoint     string
	Credenciales aws.CredentialsProvider
}

// S3 es el Almacen sobre Amazon S3.
type S3 struct {
	cliente *s3.Client
	firma   *s3.PresignClient
	bucket  string
}

// NewS3 arma el cliente con la cadena de credenciales de AWS.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, errors.New("imagenes: faltan el bucket o la región")
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region), awsconfig.WithRetryMaxAttempts(2)}
	if cfg.Credenciales != nil {
		opts = append(opts, awsconfig.WithCredentialsProvider(cfg.Credenciales))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("imagenes: configuración de AWS: %w", err)
	}
	cliente := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})
	return &S3{cliente: cliente, firma: s3.NewPresignClient(cliente), bucket: cfg.Bucket}, nil
}

// FirmarSubida firma el formulario con la política: clave exacta, Content-Type exacto y
// tamaño de 1 byte a maxBytes. S3 rechaza lo demás con 403 y la API ni se entera.
func (a *S3) FirmarSubida(ctx context.Context, clave, tipo string, maxBytes int64, vence time.Duration) (Subida, error) {
	req, err := a.firma.PresignPostObject(ctx, &s3.PutObjectInput{Bucket: aws.String(a.bucket), Key: aws.String(clave)},
		func(o *s3.PresignPostOptions) {
			o.Expires = vence
			o.Conditions = []any{
				map[string]string{"Content-Type": tipo},
				[]any{"content-length-range", 1, maxBytes},
			}
		})
	if err != nil {
		return Subida{}, fmt.Errorf("imagenes: firmar subida: %w", err)
	}
	campos := make(map[string]string, len(req.Values)+1)
	for k, v := range req.Values {
		campos[k] = v
	}
	campos["Content-Type"] = tipo
	return Subida{URL: req.URL, Campos: campos}, nil
}

// Leer baja el original; lee hasta max+1 bytes para detectar uno más grande sin cargarlo todo.
func (a *S3) Leer(ctx context.Context, clave string, max int64) ([]byte, error) {
	out, err := a.cliente.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(a.bucket), Key: aws.String(clave)})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "AccessDenied") {
			return nil, ErrSinObjeto
		}
		return nil, fmt.Errorf("imagenes: leer: %w", err)
	}
	defer func() { _ = out.Body.Close() }()
	datos, err := io.ReadAll(io.LimitReader(out.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("imagenes: leer: %w", err)
	}
	if int64(len(datos)) > max {
		return nil, ErrObjetoGrande
	}
	return datos, nil
}

// Guardar escribe un tamaño publicado con caché de un año.
func (a *S3) Guardar(ctx context.Context, clave, tipo string, datos []byte) error {
	_, err := a.cliente.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(a.bucket),
		Key:          aws.String(clave),
		Body:         bytes.NewReader(datos),
		ContentType:  aws.String(tipo),
		CacheControl: aws.String(CacheInmutable),
	})
	if err != nil {
		return fmt.Errorf("imagenes: guardar: %w", err)
	}
	return nil
}

// Borrar elimina cada clave (S3 responde 204 aunque no exista).
func (a *S3) Borrar(ctx context.Context, claves ...string) error {
	for _, clave := range claves {
		if _, err := a.cliente.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(a.bucket), Key: aws.String(clave)}); err != nil {
			return fmt.Errorf("imagenes: borrar: %w", err)
		}
	}
	return nil
}
