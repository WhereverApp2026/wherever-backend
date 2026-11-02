// Package neo4j crea el driver de Neo4j con tiempos cortos.
package neo4j

import (
	"context"
	"net/url"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	neo4jconfig "github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

const (
	socketTimeout     = 2 * time.Second
	acquireTimeout    = 2 * time.Second
	connectionTimeout = 2 * time.Second
)

// Driver envuelve al driver oficial para exponer Ping y Close.
type Driver struct {
	neo4j.DriverWithContext
}

// New crea el driver. Las credenciales, si las hay, viajan en la parte de usuario de la URI
// para no sumar claves de entorno; se separan antes de pasarla al driver.
func New(uri string) (*Driver, error) {
	target, auth, err := splitAuth(uri)
	if err != nil {
		return nil, err
	}
	driver, err := neo4j.NewDriverWithContext(target, auth, func(c *neo4jconfig.Config) {
		c.SocketConnectTimeout = socketTimeout
		c.ConnectionAcquisitionTimeout = acquireTimeout
		c.ConnectionLivenessCheckTimeout = connectionTimeout
	})
	if err != nil {
		return nil, err
	}
	return &Driver{DriverWithContext: driver}, nil
}

func splitAuth(raw string) (string, neo4j.AuthToken, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", neo4j.NoAuth(), err
	}
	if u.User == nil {
		return raw, neo4j.NoAuth(), nil
	}
	password, _ := u.User.Password()
	auth := neo4j.BasicAuth(u.User.Username(), password, "")
	u.User = nil
	return u.String(), auth, nil
}

// Ping confirma que el servidor acepta conexiones.
func (d *Driver) Ping(ctx context.Context) error {
	return d.VerifyConnectivity(ctx)
}

// Close cierra el driver.
func (d *Driver) Close(ctx context.Context) error {
	return d.DriverWithContext.Close(ctx)
}
