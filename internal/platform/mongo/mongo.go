// Package mongo crea el cliente de MongoDB con tiempos cortos.
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

const (
	connectTimeout         = 2 * time.Second
	serverSelectionTimeout = 2 * time.Second
	defaultDatabase        = "wherever"
)

// Client envuelve al cliente oficial para exponer Ping, Close y la base de la URI.
type Client struct {
	*mongo.Client
	database string
}

// New crea el cliente. No espera a la base: la conexión se valida en /ready.
// La base es la de la ruta de la URI (`mongodb://host/wherever`); sin ruta, `wherever`.
func New(uri string) (*Client, error) {
	cs, err := connstring.ParseAndValidate(uri)
	if err != nil {
		return nil, err
	}
	opts := options.Client().
		ApplyURI(uri).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(serverSelectionTimeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	database := cs.Database
	if database == "" {
		database = defaultDatabase
	}
	return &Client{Client: client, database: database}, nil
}

// DB devuelve la base de la aplicación.
func (c *Client) DB() *mongo.Database {
	return c.Database(c.database)
}

// Ping confirma que el primario responde.
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx, readpref.Primary())
}

// Close cierra las conexiones.
func (c *Client) Close(ctx context.Context) error {
	return c.Disconnect(ctx)
}
