#!/bin/bash

set -e
shopt -s nullglob

#
# Define default Variables.
#
USER="named"
GROUP="named"
COMMAND_OPTIONS_DEFAULT="-f"
NAMED_UID_DEFAULT="1000"
NAMED_GID_DEFAULT="101"
COMMAND="/usr/sbin/named -d 3 -u ${USER} -c named.conf ${COMMAND_OPTIONS:=${COMMAND_OPTIONS_DEFAULT}}"

NAMED_UID_ACTUAL=$(id -u ${USER})
NAMED_GID_ACTUAL=$(id -g ${GROUP})

CION_CONF_ROOT="${CION_CONF_ROOT:-/etc/bind}"
CION_ZONE_PATH="${CION_ZONE_PATH:-/var/bind/dyn}"
CION_ROOT_DOMAIN="${CION_ROOT_DOMAIN:-foo.bar}"
CION_WEB_ADDRESS="${CION_WEB_ADDRESS:-127.0.0.1}"
CION_NS1_HOSTNAME="${CION_NS1_HOSTNAME:-ns1}"
CION_NS2_HOSTNAME="${CION_NS2_HOSTNAME:-ns2}"
CION_NS1_ADDRESS="${CION_NS1_ADDRESS:-127.0.0.1}"
CION_NS2_ADDRESS="${CION_NS2_ADDRESS:-127.0.0.1}"
CION_TTL="${CION_TTL:-180}"

#
# Display settings on standard out.
#
echo "named settings"
echo "=============="
echo
echo "  Username:        ${USER}"
echo "  Groupname:       ${GROUP}"
echo "  UID actual:      ${NAMED_UID_ACTUAL}"
echo "  GID actual:      ${NAMED_GID_ACTUAL}"
echo "  UID prefered:    ${NAMED_UID:=${NAMED_UID_DEFAULT}}"
echo "  GID prefered:    ${NAMED_GID:=${NAMED_GID_DEFAULT}}"
echo "  Command:         ${COMMAND}"
echo

#
# Change UID / GID of named user.
#
echo "Updating UID / GID... "
if [[ ! ${NAMED_GID_ACTUAL} == ${NAMED_GID} ]] || [[ ${NAMED_UID_ACTUAL} -ne ${NAMED_UID} ]]; then
    echo "change user / group"
    deluser ${USER}
    addgroup -g ${NAMED_GID} ${GROUP}
    adduser -u ${NAMED_UID} -G ${GROUP} -h /etc/bind -g 'Linux User named' -s /sbin/nologin -D ${USER}
    echo "[DONE]"
    echo "Set owner and permissions for old uid/gid files"
    find / -user ${NAMED_UID_ACTUAL} -exec chown ${USER} {} \;
    find / -group ${NAMED_GID_ACTUAL} -exec chgrp ${GROUP} {} \;
    echo "%sudo	ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
    addgroup sudo
    adduser ${USER} sudo
    echo "[DONE]"
else
    echo "[NOTHING DONE]"
fi

#
# Create Zone directories
#
echo "Create zone directories... "
mkdir -p /var/bind/dyn

#
# Set owner and permissions.
#
echo "Set owner and permissions... "
chown -R ${USER}:${GROUP} /var/bind /etc/bind /var/run/named
chmod -R o-rwx /var/bind /etc/bind /var/run/named
echo "[DONE]"

#
# Create root-domain zone if it doesn't yet exist
#
echo "Checking for root-domain zonefile..."
configfile="${CION_CONF_ROOT}/named.config.rootzone"
zonefile="${CION_ZONE_PATH}/${CION_ROOT_DOMAIN}.zone"
if [[ -f "${zonefile}" ]]; then
    echo "Found existing root-domain zonefile, skipping!"
else
    echo "Creating root-domain zonefile... [${zonefile}]"
    timestamp="$(date +%Y%m%d)"
    (
        echo "\$TTL ${CION_TTL}"
        echo "@       IN      SOA     ${CION_NS1_HOSTNAME}.${CION_ROOT_DOMAIN}. hostmaster.${CION_ROOT_DOMAIN}.  ("
        echo "        ${timestamp}01 ; Serial"
        echo "        28800      ; Refresh"
        echo "        14400      ; Retry"
        echo "        604800     ; Expire - 1 week"
        echo "        86400 )    ; Minimum"
        echo "@ IN      NS      ${CION_NS1_HOSTNAME}"
        echo "@ IN      NS      ${CION_NS2_HOSTNAME}"
        echo "@ IN      A       ${CION_WEB_ADDRESS}"
        echo "${CION_NS1_HOSTNAME} IN	A	${CION_NS1_ADDRESS}"
        echo "${CION_NS2_HOSTNAME} IN	A	${CION_NS2_ADDRESS}"
    ) > "${zonefile}"
    chown -R named. "${CION_ZONE_PATH}"
fi

echo "Checking for root-domain configfile..."
configfile="${CION_CONF_ROOT}/named.conf.rootzone"
if [[ -f "${configfile}" ]]; then
    echo "Found existing root-domain configfile, skipping!"
else
    echo "Creating root-domain configfile... [${configfile}]"
    (
        echo "zone \"${CION_ROOT_DOMAIN}\" IN {"
        echo "  type master;"
        echo "  file \"${zonefile}\";"
        echo "  allow-transfer { 127.0.0.1; ${CION_NS2_ADDRESS}; };"
        echo "  allow-update { key rndc-key; };"
        echo "  notify yes;"
        echo "};"
    ) > "${configfile}"
    chown named. "${configfile}"
fi

#
# Generate RNDC key
#
echo "Checking existence of RNDC key..."
if [[ -f ${CION_CONF_ROOT}/named.conf.rndc ]]; then
    echo "Found RNDC key, skipping!"
else
    echo "Generating new HMAC-SHA512 key..."
    set +e
    tsig-keygen rndc-key > "${CION_CONF_ROOT}/named.conf.rndc"
    set -e
fi

#
# Patch cion-tool.sh
#
echo "Patch cion-tool.sh to have the right defaults..."
if [[ -n "${CION_WEB_PORT}" ]]; then
    SED_CION_WEB_URL="${CION_WEB_PROTO}:\/\/${CION_WEB_ADDRESS}:${CION_WEB_PORT}"
else
    SED_CION_WEB_URL="${CION_WEB_PROTO}:\/\/${CION_WEB_ADDRESS}"
fi
sed -e "s/TPL_CION_WEB_URL/${SED_CION_WEB_URL}/" /docker/cion-tool.sh > /public/cion-tool.sh
echo "[DONE]"

#
# Start named.
#
echo "Start named... "
cion serve &
exec ${COMMAND}
