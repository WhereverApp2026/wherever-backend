#!/usr/bin/env bash
# Verificación de SOLO LECTURA del entorno de desarrollo (spec nv-07). Falla (exit 1) si algo
# sale del Free Tier o queda expuesto. No crea, cambia ni borra nada.
#   uso: verificar.sh --profile wherever-dev --region us-east-2 --instance i-… [--url https://…] [--ssh ec2-user@ip --key ~/.ssh/llave]
set -uo pipefail

profile="" region="" instance="" url="" remote="" key=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		--profile) profile="$2"; shift 2 ;;
		--region) region="$2"; shift 2 ;;
		--instance) instance="$2"; shift 2 ;;
		--url) url="$2"; shift 2 ;;
		--ssh) remote="$2"; shift 2 ;;
		--key) key="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done
[[ -n "$profile" && -n "$region" && -n "$instance" ]] || { echo "uso: $0 --profile <perfil> --region <región> --instance <id> [--url <https://…>] [--ssh <usuario@ip> --key <llave>]" >&2; exit 2; }

# tr: en Windows la CLI termina las líneas en CRLF y el último campo quedaría con \r.
aws_q() { aws --profile "$profile" --region "$region" --output text "$@" | tr -d '\r'; }
fails=0
ok() { echo "  OK    $1"; }
bad() { echo "  FALLA $1"; fails=$((fails + 1)); }
check() { if [[ "$2" == "$3" ]]; then ok "$1"; else bad "$1 (es '$2', debe ser '$3')"; fi; }

echo "verificar: instancia $instance en $region"
type="$(aws_q ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].InstanceType')"
eligible="$(aws_q ec2 describe-instance-types --instance-types "$type" --query 'InstanceTypes[0].FreeTierEligible')"
check "tipo $type elegible para el Free Tier" "$eligible" "True"
check "IMDSv2 obligatorio" "$(aws_q ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].MetadataOptions.HttpTokens')" "required"
credits="$(aws_q ec2 describe-instance-credit-specifications --instance-ids "$instance" --query 'InstanceCreditSpecifications[0].CpuCredits')"
check "créditos de CPU" "$credits" "standard"

vol="$(aws_q ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].BlockDeviceMappings[0].Ebs.VolumeId')"
read -r size encrypted vtype < <(aws_q ec2 describe-volumes --volume-ids "$vol" --query 'Volumes[0].[Size,Encrypted,VolumeType]')
if (( size <= 30 )); then ok "disco de $size GB (≤ 30)"; else bad "disco de $size GB (> 30)"; fi
check "disco cifrado" "$encrypted" "True"
check "disco gp3" "$vtype" "gp3"

sg="$(aws_q ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].SecurityGroups[0].GroupId')"
rules="$(aws_q ec2 describe-security-group-rules --filters "Name=group-id,Values=$sg" --query 'SecurityGroupRules[?IsEgress==`false`].[FromPort,ToPort,CidrIpv4]')"
while read -r from to cidr; do
	[[ -z "$from" ]] && continue
	case "$from" in
		80 | 443) ok "puerto $from abierto (público, esperado)" ;;
		22) if [[ "$cidr" == "0.0.0.0/0" ]]; then bad "SSH abierto a todo internet"; else ok "SSH solo desde $cidr"; fi ;;
		# MongoDB vive en Atlas, pero si alguien abre 27017 aquí también es un error.
		27017 | 6379 | 7687 | 7474) bad "puerto de base de datos $from expuesto ($cidr)" ;;
		*) if [[ "$cidr" == "0.0.0.0/0" ]]; then bad "puerto $from-$to abierto a todo internet"; else ok "puerto $from solo desde $cidr"; fi ;;
	esac
done <<< "$rules"

associated="$(aws_q ec2 describe-addresses --query 'length(Addresses[?InstanceId==`'"$instance"'`])')"
loose="$(aws_q ec2 describe-addresses --query 'length(Addresses[?AssociationId==null])')"
check "IP elástica asociada a la instancia" "$associated" "1"
check "IPs elásticas sueltas (cobran)" "$loose" "0"
check "NAT gateways" "$(aws_q ec2 describe-nat-gateways --filter Name=state,Values=available,pending --query 'length(NatGateways)')" "0"
check "balanceadores" "$(aws_q elbv2 describe-load-balancers --query 'length(LoadBalancers)' 2>/dev/null || echo 0)" "0"

for sched in wherever-dev-apagar wherever-dev-encender; do
	state="$(aws_q scheduler get-schedule --name "$sched" --query 'State' 2>/dev/null || echo FALTA)"
	check "regla $sched" "$state" "ENABLED"
done
account="$(aws_q sts get-caller-identity --query Account)"
# describe-budget (uno, por nombre): la política wherever-dev-deployer solo deja ver wherever-dev-*.
budget="$(aws --profile "$profile" --output text budgets describe-budget --account-id "$account" --budget-name wherever-dev-mensual --query 'Budget.BudgetName' 2>/dev/null || echo FALTA)"
check "presupuesto wherever-dev-mensual" "$budget" "wherever-dev-mensual"
caller="$(aws_q sts get-caller-identity --query Arn)"
if [[ "$caller" == *":root" ]]; then bad "el perfil usa el usuario raíz ($caller)"; else ok "perfil sin raíz ($caller)"; fi

state="$(aws_q ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].State.Name')"
echo "  INFO  instancia $state"
if [[ -n "$url" && "$state" == "running" ]]; then
	# Sin `|| echo 000`: -w ya imprime 000 si falla, y en Windows curl puede salir con error tras un 200.
	code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 "$url/ready" || true)"
	check "HTTPS $url/ready con certificado válido" "$code" "200"
	# 200 también puede ser `degraded`: Redis caído o sin cuota (nv-11).
	estado="$(curl -s --max-time 15 "$url/ready" | sed -n 's/.*"estado":"\([^"]*\)".*/\1/p')"
	check "$url/ready con todas las bases" "$estado" "ok"
fi

# Redis y Neo4j viven fuera de la instancia (nv-11).
if [[ -n "$remote" && "$state" == "running" ]]; then
	ssh_opts=(-o BatchMode=yes -o ConnectTimeout=15 -o LogLevel=ERROR)
	[[ -n "$key" ]] && ssh_opts+=(-i "$key")
	if inside="$(ssh "${ssh_opts[@]}" "$remote" "sudo ss -ltnH; docker ps --format '{{.Names}}'" | tr -d '\r')"; then
		if grep -qE ':(6379|7687|7474)\b|redis|neo4j' <<<"$inside"; then
			bad "Redis o Neo4j siguen en la instancia"
		else
			ok "sin Redis ni Neo4j en la instancia"
		fi
	else
		bad "no se pudo entrar por SSH a $remote"
	fi
fi

if (( fails > 0 )); then
	echo "verificar: FALLA ($fails)"
	exit 1
fi
echo "verificar: OK"
