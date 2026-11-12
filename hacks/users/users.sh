#!/bin/bash
# List Superphenix users and activate or deactivate them.
#
# Talks to the API database (table "users"). Login is allowed only when
# is_active is true and deleted_at is null. Soft-deleted rows are shown
# but cannot be toggled.
#
# Connection uses DATABASE_URL when set. The toolbox Deployment sets it from
# the postgres Secret uri key when autoConnect is on, or builds it from
# config.database otherwise, so spx-users needs no credentials.
# With none set, it targets the local compose database from hacks/compose
# (localhost:5433, db superphenix).
#
#   DATABASE_URL
#   PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE
#   SUPERPHENIX_DB_USER SUPERPHENIX_DB_PASSWORD SUPERPHENIX_DB_NAME
#
# Commands at the prompt:
#   l            list every user
#   a            list active users
#   i            list inactive users
#   d            list soft-deleted users
#   t <n|email>  activate or deactivate
#   <n>          same as t <n>, using the number from the current list
#   q            quit

set -euo pipefail

RST=$'\033[0m'
BRed=$'\033[1;31m'
BGreen=$'\033[1;32m'
BYellow=$'\033[1;33m'
BBlue=$'\033[1;34m'
BWhite=$'\033[1;37m'

msg_warn() { echo -ne "$BWhite[$BRed!$BWhite]$RST $1"; }
msg_query() { echo -ne "$BWhite[$BBlue?$BWhite]$RST $1"; }
msg_info() { echo -ne "$BWhite[$BYellow~$BWhite]$RST $1"; }
msg_action() { echo -ne "$BWhite[$BGreen+$BWhite]$RST $1"; }

if ! command -v psql >/dev/null 2>&1; then
	msg_warn "psql is required\n"
	exit 1
fi

if [[ -z "${PGHOST:-}" && -z "${DATABASE_URL:-}" ]]; then
	export PGHOST=localhost
	export PGPORT="${PGPORT:-5433}"
	export PGUSER="${PGUSER:-${SUPERPHENIX_DB_USER:-superphenix}}"
	export PGPASSWORD="${PGPASSWORD:-${SUPERPHENIX_DB_PASSWORD:-secret}}"
	export PGDATABASE="${PGDATABASE:-${SUPERPHENIX_DB_NAME:-superphenix}}"
	export PGSSLMODE="${PGSSLMODE:-disable}"
fi

psql_q() {
	# libpq does not read DATABASE_URL. Pass it as the connection string.
	if [[ -n "${DATABASE_URL:-}" ]]; then
		psql "$DATABASE_URL" -X --no-psqlrc -v ON_ERROR_STOP=1 -A -F $'\t' -t -P pager=off "$@"
	else
		psql -X --no-psqlrc -v ON_ERROR_STOP=1 -A -F $'\t' -t -P pager=off "$@"
	fi
}

if ! psql_q -c 'SELECT 1 FROM users LIMIT 1' >/dev/null; then
	msg_warn "Cannot read the users table. Check the database connection.\n"
	exit 1
fi

FILTER="all"
IDS=()
STATUSES=()
EMAILS=()

where_clause() {
	case "$1" in
	all) echo "TRUE" ;;
	active) echo "deleted_at IS NULL AND is_active" ;;
	inactive) echo "deleted_at IS NULL AND NOT is_active" ;;
	deleted) echo "deleted_at IS NOT NULL" ;;
	*) echo "TRUE" ;;
	esac
}

status_color() {
	case "$1" in
	active) printf '%s' "$BGreen" ;;
	inactive) printf '%s' "$BYellow" ;;
	deleted) printf '%s' "$BRed" ;;
	*) printf '%s' "$RST" ;;
	esac
}

load_users() {
	local where raw id status email name provider created
	where="$(where_clause "$FILTER")"
	raw="$(psql_q -c "
		SELECT id,
		       CASE
		         WHEN deleted_at IS NOT NULL THEN 'deleted'
		         WHEN is_active THEN 'active'
		         ELSE 'inactive'
		       END,
		       email,
		       COALESCE(NULLIF(btrim(concat_ws(' ', firstname, lastname)), ''), '-'),
		       COALESCE(NULLIF(provider, ''), '-'),
		       to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI')
		FROM users
		WHERE ${where}
		ORDER BY (deleted_at IS NOT NULL), is_active DESC, email
	")"

	IDS=()
	STATUSES=()
	EMAILS=()
	ROWS=()

	while IFS=$'\t' read -r id status email name provider created; do
		[[ -z "${id:-}" ]] && continue
		IDS+=("$id")
		STATUSES+=("$status")
		EMAILS+=("$email")
		ROWS+=("$(printf '%s\t%s\t%s\t%s' "$email" "$name" "$provider" "$created")")
	done <<<"$raw"
}

print_summary() {
	local counts active inactive deleted
	counts="$(psql_q -c "
		SELECT count(*) FILTER (WHERE deleted_at IS NULL AND is_active),
		       count(*) FILTER (WHERE deleted_at IS NULL AND NOT is_active),
		       count(*) FILTER (WHERE deleted_at IS NOT NULL)
		FROM users
	")"
	IFS=$'\t' read -r active inactive deleted <<<"$counts"
	msg_info "Users: ${BGreen}${active} active${RST}, ${BYellow}${inactive} inactive${RST}, ${BRed}${deleted} deleted${RST}. Showing ${FILTER}.\n"
}

print_list() {
	local i email name provider created color
	local -a emails=() names=() providers=() createds=()
	local w_num w_status w_email w_name w_provider
	echo
	print_summary
	if [[ ${#IDS[@]} -eq 0 ]]; then
		msg_info "No users in this view.\n"
		return
	fi

	w_num=${#IDS[@]}
	w_num=${#w_num}
	w_status=6
	w_email=5
	w_name=4
	w_provider=8
	for i in "${!IDS[@]}"; do
		IFS=$'\t' read -r email name provider created <<<"${ROWS[$i]}"
		emails+=("$email")
		names+=("$name")
		providers+=("$provider")
		createds+=("$created")
		((${#STATUSES[$i]} > w_status)) && w_status=${#STATUSES[$i]}
		((${#email} > w_email)) && w_email=${#email}
		((${#name} > w_name)) && w_name=${#name}
		((${#provider} > w_provider)) && w_provider=${#provider}
	done

	printf "%${w_num}s  %-${w_status}s  %-${w_email}s  %-${w_name}s  %-${w_provider}s  %s\n" \
		"#" "status" "email" "name" "provider" "created (UTC)"
	for i in "${!IDS[@]}"; do
		color="$(status_color "${STATUSES[$i]}")"
		printf "%${w_num}d  %s%-${w_status}s%s  %-${w_email}s  %-${w_name}s  %-${w_provider}s  %s\n" \
			"$((i + 1))" "$color" "${STATUSES[$i]}" "$RST" \
			"${emails[$i]}" "${names[$i]}" "${providers[$i]}" "${createds[$i]}"
	done
}

confirm() {
	local reply
	msg_query "$1 [y/N] "
	read -r reply
	[[ "$reply" == "y" || "$reply" == "Y" ]]
}

set_active() {
	local id="$1" value="$2" result email state
	# SQL with :'var' must be read by psql itself. -c forwards the text
	# unchanged, so Postgres rejects the colon.
	result="$(psql_q -v id="$id" -v active="$value" <<SQL
UPDATE users
SET is_active = :'active'::boolean, updated_at = NOW()
WHERE id = :'id'::uuid AND deleted_at IS NULL
RETURNING email, is_active
SQL
)"
	if [[ -z "$result" ]]; then
		msg_warn "No live user with that id. Deleted users stay deleted.\n"
		return 1
	fi
	IFS=$'\t' read -r email state <<<"$result"
	if [[ "$state" == "t" ]]; then
		msg_action "Activated ${email}\n"
	else
		msg_action "Deactivated ${email}\n"
	fi
}

resolve_target() {
	local target="$1" n matches id email status
	RESOLVED_ID=""
	RESOLVED_STATUS=""
	RESOLVED_EMAIL=""

	if [[ "$target" =~ ^[0-9]+$ ]]; then
		n=$((target))
		if ((n < 1 || n > ${#IDS[@]})); then
			msg_warn "No user numbered ${target} in the current list.\n"
			return 1
		fi
		RESOLVED_ID="${IDS[$((n - 1))]}"
		RESOLVED_STATUS="${STATUSES[$((n - 1))]}"
		RESOLVED_EMAIL="${EMAILS[$((n - 1))]}"
		return 0
	fi

	if [[ "$target" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
		matches="$(psql_q -v id="$target" <<SQL
SELECT id,
       CASE WHEN deleted_at IS NOT NULL THEN 'deleted' WHEN is_active THEN 'active' ELSE 'inactive' END,
       email
FROM users
WHERE id = :'id'::uuid
SQL
)"
	else
		matches="$(psql_q -v email="$target" <<SQL
SELECT id,
       CASE WHEN deleted_at IS NOT NULL THEN 'deleted' WHEN is_active THEN 'active' ELSE 'inactive' END,
       email
FROM users
WHERE lower(email) = lower(:'email')
SQL
)"
	fi

	if [[ -z "$matches" ]]; then
		msg_warn "No user matches '${target}'.\n"
		return 1
	fi
	if [[ "$(printf '%s\n' "$matches" | wc -l | tr -d ' ')" -gt 1 ]]; then
		msg_warn "Several users match '${target}'. Use the list number or the id:\n"
		printf '%s\n' "$matches" | while IFS=$'\t' read -r id status email; do
			printf '  %s  %s  %s\n' "$status" "$email" "$id"
		done
		return 1
	fi
	IFS=$'\t' read -r RESOLVED_ID RESOLVED_STATUS RESOLVED_EMAIL <<<"$matches"
}

toggle_user() {
	local target="$1" next label
	if [[ -z "$target" ]]; then
		msg_query "User number, email, or id: "
		read -r target
	fi
	[[ -z "$target" ]] && return 0
	resolve_target "$target" || return 0
	if [[ "$RESOLVED_STATUS" == "deleted" ]]; then
		msg_warn "${RESOLVED_EMAIL} is deleted and cannot be activated.\n"
		return 0
	fi
	if [[ "$RESOLVED_STATUS" == "active" ]]; then
		next="false"
		label="Deactivate"
	else
		next="true"
		label="Activate"
	fi
	confirm "${label} ${RESOLVED_EMAIL}?" || return 0
	set_active "$RESOLVED_ID" "$next" || true
}

if [[ -n "${DATABASE_URL:-}" ]]; then
	db_display="${DATABASE_URL#*://}"
	db_display="${db_display#*@}"
	db_display="${db_display%%\?*}"
else
	db_display="${PGUSER:-}@${PGHOST:-url}:${PGPORT:-}/${PGDATABASE:-}"
fi

echo
msg_info "Superphenix user activation. Database: ${db_display}\n"

while true; do
	load_users
	print_list
	echo
	msg_query "Command ([l]ist [a]ctive [i]nactive [d]eleted [t]oggle [q]uit): "
	read -r action target || exit 0
	case "${action:-}" in
	q | quit | exit) exit 0 ;;
	l | list | "") FILTER="all" ;;
	a | active) FILTER="active" ;;
	i | inactive) FILTER="inactive" ;;
	d | deleted) FILTER="deleted" ;;
	t | toggle) toggle_user "${target:-}" ;;
	*)
		if [[ "$action" =~ ^[0-9]+$ && -z "${target:-}" ]]; then
			toggle_user "$action"
		else
			msg_warn "Unknown command '${action}'.\n"
		fi
		;;
	esac
done
