#!/bin/bash


CION_WEB_URL="${CION_WEB_URL:-TPL_CION_WEB_URL}"
#CION_WEB_URL="${CION_WEB_URL:-http://127.0.0.1:1234}"

# Usage info
show_help() {
cat << EOFHELP
Usage: ${0##*/} [global options] command [options] [argument]...
Commands
        help
        version
        register [-w] zonename
        update zonename item value
        deletezone zonename
EOFHELP
}

show_version() {
cat << EOFVERSION
${0##*/} version 0.0.1
EOFVERSION
}

# register
register_namespace() {
# store the whole response with the status at the and
HTTP_RESPONSE=$(curl -s \
    -w '\n%{http_code}' \
    -X PUT \
    -H "Accept: application/json; version=1.0.0" \
    -H "Content-Type: application/json" \
    -d "{\"zone\": \"${1}\"}" \
    ${CION_WEB_URL}/register)

# extract the body
HTTP_BODY=$(echo "$HTTP_RESPONSE" | sed \$d)

# extract the status
HTTP_STATUS=$(echo "$HTTP_RESPONSE" | tail -n 1)

# print the body
#echo "BODY: $HTTP_BODY"
#echo "STATUS: *$HTTP_STATUS*"

# example using the status
case "$HTTP_STATUS" in
    202)    #echo "Domain created. Access token:"
            TOKEN=$(echo "$HTTP_BODY" | sed -e 's/[":,{}]//g' -e 's/\([a-z ]*\)auth_key//')
            echo ${TOKEN}
            exit 0
            ;;
    423)    echo "Domain already taken."
            exit 11
            ;;
    429)    TIME=$(echo "$HTTP_BODY" | sed -e 's/[":{}]//g' -e 's/\([a-z ]*\)\([0-9]\{1,2\}h[0-9]\{1,2\}m[0-9]\{1,2\}s\)/\2/' -e 's/[mh]/:/g' -e 's/s//')
            
            
            echo "Time to wait for next registration: ${TIME}"

            if [[ -n ${2} ]]; then
                sleep $(TZ="UTC" date -d "1970-01-01 ${TIME}" +%s)
                register_namespace ${1} 1
            fi

            exit 12
            ;;
    *)  echo "wut? $HTTP_STATUS"
        exit 13
        ;;
esac
    
}

##################################
# Commandline processing
##################################

# begin
if [[ $# -eq 0 ]]; then
    echo "ERROR: no arguments given."
    show_help
    exit 1
fi

# global options
while getopts ":w-:" opt; do
    case $opt in
        w)
            OPT_REGWAIT=1
            shift $((OPTIND-1))
            ;;
        -)
            ;;
        \?)
            echo "[ERROR] Invalid option: -${OPTARG}"
            show_help
            exit 3
            ;;
    esac
done

# get the command
CMD=${1}
shift

case ${CMD} in
    help | --help )
        show_help
        exit 0
        ;;
    version | --version )
        show_version
        exit 0
        ;;
    register )
        while getopts ":w" opt; do
            case $opt in
                w)
                    OPT_REGWAIT=1
                    shift $((OPTIND-1))
                    ;;
                \?)
                    echo "[ERROR] Invalid option: -${OPTARG}"
                    show_help
                    exit 3
                    ;;
            esac
        done
        if [[ $# -gt 1 ]]; then
            echo "[ERROR] Extra arguments given."
            show_help
            exit 4
        fi
        if [[ $# -eq 0 ]]; then
            echo "[ERROR] No domain name given."
            show_help
            exit 5
        fi
        register_namespace ${1} ${OPT_REGWAIT}
        ;;

    *)  echo "ERROR: Unrecognized commnd '${CMD}'"
        show_help
        exit 2
        ;;
esac

exit 255
