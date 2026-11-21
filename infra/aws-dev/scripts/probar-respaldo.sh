#!/usr/bin/env bash
# Prueba de respaldo-mongo.sh sin AWS (job iac-scan de ci-pr.yml; en local, en Linux o WSL):
#   1. shellcheck de los scripts del servidor y validación del calendario del timer;
#   2. respaldo en seco contra un MongoDB en Docker con usuario y contraseña;
#   3. restauración del archivo con mongorestore en otra base y conteo de documentos;
#   4. una cadena inalcanzable termina en error, con el evento `error` y sin archivo.
# En los pasos 2 y 4 la clave de prueba no debe aparecer en la salida.
#   uso: probar-respaldo.sh
# `cond && ok || bad`: ok y bad nunca fallan, así que bad corre solo si cond falla.
# shellcheck disable=SC2015
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
server="$(cd "$here/../server" && pwd)"
script="$here/respaldo-mongo.sh"
shellcheck_image=koalaman/shellcheck:v0.11.0@sha256:61862eba1fcf09a484ebcc6feea46f1782532571a34ed51fedf90dd25f925a8d
image="$(sed -n 's/^mongo_image=//p' "$script")"
net="respaldo-prueba-$$"
db="respaldo-prueba-mongo-$$"
user=prueba
clave="clave-$(od -An -tx1 -N12 /dev/urandom | tr -d ' \n')"
work="$(mktemp -d)"
fails=0

ok() { echo "  ok    $1"; }
bad() { echo "  FALLA $1"; fails=$((fails + 1)); }

cleanup() {
	docker rm -f "$db" >/dev/null 2>&1 || true
	docker network rm "$net" >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT

echo "probar-respaldo: shellcheck"
scripts=(respaldo-mongo.sh probar-respaldo.sh instalar-actualizador.sh crear.sh rotar-secretos.sh)
if command -v shellcheck >/dev/null; then
	(cd "$here" && shellcheck "${scripts[@]}") && ok "shellcheck" || bad "shellcheck"
else
	docker run --rm -v "$here:/mnt:ro" -w /mnt "$shellcheck_image" "${scripts[@]}" && ok "shellcheck" || bad "shellcheck"
fi

calendar="$(sed -n 's/^OnCalendar=//p' "$server/wherever-respaldo.timer")"
if command -v systemd-analyze >/dev/null; then
	systemd-analyze calendar "$calendar" >/dev/null && ok "OnCalendar válido: $calendar" || bad "OnCalendar inválido: $calendar"
fi

echo "probar-respaldo: MongoDB de prueba ($image)"
docker network create "$net" >/dev/null
# La descarga va aparte para que no cuente en la espera del arranque.
docker pull -q "$image" >/dev/null
mongosh() { docker exec "$db" mongosh --quiet -u "$user" -p "$clave" --authenticationDatabase admin "$@"; }
# Arranca MongoDB y espera el ping. Si el contenedor muere (pasó en un runner de GitHub, corrida
# 297), muestra sus logs y lo intenta una vez más; si tampoco, la prueba falla con la causa a la vista.
start_mongo() {
	docker rm -f "$db" >/dev/null 2>&1 || true
	docker run -d --name "$db" --network "$net" -e MONGO_INITDB_ROOT_USERNAME="$user" \
		-e MONGO_INITDB_ROOT_PASSWORD="$clave" "$image" >/dev/null
	for _ in $(seq 1 90); do
		mongosh --eval 'db.runCommand({ping: 1}).ok' 2>/dev/null | grep -qx 1 && return 0
		[[ "$(docker inspect -f '{{.State.Running}}' "$db" 2>/dev/null)" == "true" ]] || break
		sleep 1
	done
	echo "probar-respaldo: MongoDB no quedó listo ($(docker inspect -f '{{.State.Status}} {{.State.ExitCode}}' "$db" 2>/dev/null))" >&2
	docker logs --tail 30 "$db" 2>&1 | grep -vF "$clave" >&2 || true
	return 1
}
start_mongo || start_mongo || { bad "MongoDB de prueba no arrancó"; exit 1; }
mongosh wherever --eval 'db.lugares.insertMany([{nombre: "a"}, {nombre: "b"}, {nombre: "c"}]); db.usuarios.insertOne({correo: "x@ejemplo.com"})' >/dev/null

echo "probar-respaldo: respaldo en seco"
printf 'APP_ENV=dev\nMONGO_URI=%s\n' "mongodb://$user:$clave@$db:27017/wherever?authSource=admin" >"$work/ok.env"
mkdir -p "$work/salida"
if RESPALDO_ENV_FILE="$work/ok.env" RESPALDO_RED="$net" bash "$script" --seco "$work/salida" >"$work/ok.log" 2>&1; then
	ok "terminó en 0"
else
	bad "terminó en error"; cat "$work/ok.log"
fi
grep -q '"evento":"respaldo","estado":"ok","bytes":[1-9]' "$work/ok.log" && ok "evento respaldo ok con tamaño" || bad "sin evento ok"
grep -qF "$clave" "$work/ok.log" && bad "la clave apareció en la salida" || ok "sin clave en la salida"
archive="$work/salida/$(TZ=America/Mexico_City date +%F).archive.gz"
if [[ -s "$archive" ]]; then
	ok "archivo $(basename "$archive"), $(stat -c %s "$archive") bytes"
else
	bad "no quedó el archivo"
fi

echo "probar-respaldo: restauración en otra base"
if [[ -s "$archive" ]]; then
	docker run -i --rm --network "$net" "$image" mongorestore --quiet \
		--uri="mongodb://$user:$clave@$db:27017/?authSource=admin" --archive --gzip \
		--nsFrom='wherever.*' --nsTo='verificacion.*' <"$archive" >/dev/null 2>&1 &&
		ok "mongorestore terminó" || bad "mongorestore falló"
	count="$(mongosh verificacion --eval 'db.lugares.countDocuments() + db.usuarios.countDocuments()' | tr -d '\r')"
	[[ "$count" == "4" ]] && ok "4 documentos restaurados" || bad "documentos restaurados: $count (se esperaban 4)"
fi

echo "probar-respaldo: cadena inalcanzable"
printf 'MONGO_URI=%s\n' "mongodb://$user:$clave@no-existe:27017/wherever?serverSelectionTimeoutMS=2000" >"$work/mal.env"
mkdir -p "$work/salida-mal"
if RESPALDO_ENV_FILE="$work/mal.env" RESPALDO_RED="$net" bash "$script" --seco "$work/salida-mal" >"$work/mal.log" 2>&1; then
	bad "terminó en 0 sin base"
else
	ok "terminó en error"
fi
grep -q '"evento":"respaldo","estado":"error","paso":"mongodump"' "$work/mal.log" && ok "evento error en mongodump" || { bad "sin evento error"; cat "$work/mal.log"; }
grep -q 'FALLÓ' "$work/mal.log" && ok "el log dice FALLÓ" || bad "el log no dice que falló"
grep -qF "$clave" "$work/mal.log" && bad "la clave apareció en la salida" || ok "sin clave en la salida"
[[ -z "$(ls -A "$work/salida-mal")" ]] && ok "sin archivo" || bad "dejó un archivo"

if ((fails > 0)); then
	echo "probar-respaldo: FALLA ($fails)"
	exit 1
fi
echo "probar-respaldo: OK"
