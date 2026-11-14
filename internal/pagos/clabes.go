package pagos

import (
	"context"
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// clabesExpediente lee la CLABE cifrada que el dueño guardó en el alta (nv-55).
type clabesExpediente struct{ exp *expediente.Service }

// CLABEsDe arma el puerto CLABEs con el expediente; sin expediente, nil.
func CLABEsDe(exp *expediente.Service) CLABEs {
	if exp == nil {
		return nil
	}
	return clabesExpediente{exp: exp}
}

// CuentaDeposito descifra la cuenta; sin documento devuelve ok = false.
func (c clabesExpediente) CuentaDeposito(ctx context.Context, negocio bson.ObjectID) (string, string, bool, error) {
	doc, err := c.exp.LeerDueno(ctx, negocio, expediente.TipoCuenta)
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Kind == apperr.KindNotFound {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	var cuenta expediente.Cuenta
	if err := json.Unmarshal(doc.Datos, &cuenta); err != nil || len(cuenta.CLABE) != 18 {
		return "", "", false, err
	}
	return cuenta.CLABE, cuenta.Titular, true, nil
}
