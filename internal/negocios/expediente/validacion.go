package expediente

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// rfcPattern: 3 letras (persona moral) o 4 (persona física), fecha AAMMDD y homoclave de 3.
var rfcPattern = regexp.MustCompile(`^([A-ZÑ&]{3,4})([0-9]{6})([A-Z0-9]{2}[0-9A])$`)

var cpPattern = regexp.MustCompile(`^[0-9]{5}$`)

// NormalizarRFC quita espacios y guiones y pasa a mayúsculas.
func NormalizarRFC(raw string) string {
	r := strings.NewReplacer(" ", "", "-", "")
	return strings.ToUpper(r.Replace(strings.TrimSpace(raw)))
}

// RFCValido revisa la forma del SAT: 12 caracteres (moral) o 13 (física), fecha real y
// homoclave. No consulta al SAT ni calcula el dígito verificador (un RFC antiguo puede no
// cumplirlo); la persona que revisa compara contra la constancia.
func RFCValido(rfc string) bool {
	m := rfcPattern.FindStringSubmatch(rfc)
	if m == nil {
		return false
	}
	n := len([]rune(rfc))
	if n != 12 && n != 13 {
		return false
	}
	_, err := time.Parse("060102", m[2])
	return err == nil
}

// regimenesSAT es el catálogo c_RegimenFiscal del SAT (CFDI 4.0).
var regimenesSAT = []string{"601", "603", "605", "606", "607", "608", "610", "611", "612", "614", "615", "616", "620", "621", "622", "623", "624", "625", "626"}

// usosCFDI es el catálogo c_UsoCFDI del SAT (CFDI 4.0).
var usosCFDI = []string{"G01", "G02", "G03", "I01", "I02", "I03", "I04", "I05", "I06", "I07", "I08",
	"D01", "D02", "D03", "D04", "D05", "D06", "D07", "D08", "D09", "D10", "S01", "CP01", "CN01"}

// RegimenValido dice si la clave está en c_RegimenFiscal.
func RegimenValido(clave string) bool { return slices.Contains(regimenesSAT, clave) }

// UsoCFDIValido dice si la clave está en c_UsoCFDI.
func UsoCFDIValido(clave string) bool { return slices.Contains(usosCFDI, clave) }

// CPValido dice si el código postal tiene 5 dígitos.
func CPValido(cp string) bool { return cpPattern.MatchString(cp) }

// bancos es el catálogo de instituciones de Banxico por los 3 primeros dígitos de la CLABE.
var bancos = map[string]string{
	"002": "BANAMEX", "006": "BANCOMEXT", "009": "BANOBRAS", "012": "BBVA MEXICO", "014": "SANTANDER",
	"019": "BANJERCITO", "021": "HSBC", "030": "BAJIO", "036": "INBURSA", "042": "MIFEL",
	"044": "SCOTIABANK", "058": "BANREGIO", "059": "INVEX", "060": "BANSI", "062": "AFIRME",
	"072": "BANORTE", "106": "BANK OF AMERICA", "108": "MUFG", "110": "JP MORGAN", "112": "BMONEX",
	"113": "VE POR MAS", "127": "AZTECA", "128": "AUTOFIN", "129": "BARCLAYS", "130": "COMPARTAMOS",
	"132": "MULTIVA", "133": "ACTINVER", "135": "NAFIN", "136": "INTERCAM", "137": "BANCOPPEL",
	"138": "ABC CAPITAL", "140": "CONSUBANCO", "141": "VOLKSWAGEN", "143": "CIBANCO", "145": "BBASE",
	"147": "BANKAOOL", "148": "PAGATODO", "150": "INMOBILIARIO", "151": "DONDE", "152": "BANCREA",
	"154": "BANCO COVALTO", "155": "ICBC", "156": "SABADELL", "157": "SHINHAN", "158": "MIZUHO BANK",
	"159": "BANK OF CHINA", "160": "BANCO S3", "166": "BANCO DEL BIENESTAR", "168": "HIPOTECARIA FEDERAL",
	"638": "NU MEXICO", "646": "STP", "722": "MERCADO PAGO W",
}

var clabePattern = regexp.MustCompile(`^[0-9]{18}$`)

// pesosCLABE son los pesos 3-7-1 del dígito verificador.
var pesosCLABE = [3]int{3, 7, 1}

// BancoCLABE revisa los 18 dígitos, el banco y el dígito verificador; devuelve el banco.
func BancoCLABE(clabe string) (string, bool) {
	if !clabePattern.MatchString(clabe) {
		return "", false
	}
	banco, ok := bancos[clabe[:3]]
	if !ok {
		return "", false
	}
	suma := 0
	for i := 0; i < 17; i++ {
		suma += (int(clabe[i]-'0') * pesosCLABE[i%3]) % 10
	}
	if (10-suma%10)%10 != int(clabe[17]-'0') {
		return "", false
	}
	return banco, true
}

// Enmascarar deja solo los últimos 4 dígitos.
func Enmascarar(clabe string) string {
	if len(clabe) < 4 {
		return ""
	}
	return clabe[len(clabe)-4:]
}
