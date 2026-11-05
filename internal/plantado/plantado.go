// Package plantado es una prueba de nv-09: el gate debe salir en rojo. Este PR no se fusiona.
package plantado

import (
	"os"
	"os/exec"
)

const awsKey = "AKIAQ7XW3PLANTADO9FK"

// Run ejecuta lo que llegue sin validar.
func Run() ([]byte, string, error) {
	out, err := exec.Command("sh", "-c", os.Args[1]).Output()
	return out, awsKey, err
}
