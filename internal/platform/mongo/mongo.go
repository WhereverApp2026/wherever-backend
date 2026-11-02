// Package mongo crea el cliente de MongoDB con tiempos cortos.
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	connectTimeout         = 2 * time.Second
	serverSelectionTimeout = 2 * time.Second
)

// Client envuelve al cliente oficial para exponer Ping y Close.
type Client struct {
	*mongo.Client
}

// New crea el cliente. No espera a la base: la conexión se valida en /ready.
func New(uri string) (*Client, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(serverSelectionTimeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

// Ping confirma que el primario responde.
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx, readpref.Primary())
}

// Close cierra las conexiones.
func (c *Client) Close(ctx context.Context) error {
	return c.Disconnect(ctx)
}
