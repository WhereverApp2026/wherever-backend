package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LoadDotEnv copia al entorno las claves de un archivo .env que aún no estén definidas.
// Si el archivo no existe no hace nada: en contenedores la configuración llega por entorno.
func LoadDotEnv(path string) error {
	file, err := os.Open(path) // #nosec G304 -- ruta fija elegida por main, no por quien llama a la API
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: se esperaba CLAVE=valor", path, n)
		}
		key = strings.TrimSpace(key)
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"`)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
