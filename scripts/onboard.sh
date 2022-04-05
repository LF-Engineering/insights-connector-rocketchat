#!/bin/bash
# [TOKEN="`cat ./secrets/agw-token.secret`"] [DBG=2] ./scripts/onboard.sh 'https://chat.hyperledger.org' 'aries' [./secrets/user.encrypted.secret ./secrets/token.encrypted.secret]
if [ -z "${1}" ]
then
  echo "$0: you need to specify Rocketchat server URL as a 1st argument"
  exit 1
fi
if [ -z "${2}" ]
then
  echo "$0: you need to specify Rocketchat channel as a 2nd argument"
  exit 2
fi
if ( [ ! -z "${3}" ] && [ ! -z "${4}" ] )
then
  USR="`cat ${3}`"
  TOK="`cat ${4}`"
fi
if [ -z "$TOKEN" ]
then
  TOKEN="`./scripts/get_token.sh`"
  echo "Got token: ${TOKEN}"
  echo -n "${TOKEN}" > ./secrets/agw-token.secret
fi
if [ -z "$TOKEN" ]
then
  echo "$0: no TOKEN specified, exiting"
  exit 2
else
  echo "Using provided token: ${TOKEN}"
fi
if [ -z "${DBG}" ]
then
  DBG="0"
fi
if ( [ ! -z "${USR}" ] && [ ! -z "${TOK}" ] )
then
  if [ ! -z "${DBG}" ]
  then
    echo "curl -s -XPOST -H Content-Type: application/json -H Authorization: Bearer ${TOKEN} https://api-gw.dev.platform.linuxfoundation.org/insights-service/v2/connectors/rocketchat -d{\"rocketchat_url\":\"${1}\",\"channel\":\"${2}\",\"user_id\":\"${USR}\",\"api_token\":\"${TOK}\",\"rocketchat_debug\":${DBG}} | jq -rS ."
  fi
  curl -s -XPOST -H 'Content-Type: application/json' -H "Authorization: Bearer ${TOKEN}" 'https://api-gw.dev.platform.linuxfoundation.org/insights-service/v2/connectors/rocketchat' -d"{\"rocketchat_url\":\"${1}\",\"channel\":\"${2}\",\"user_id\":\"${USR}\",\"api_token\":\"${TOK}\",\"rocketchat_debug\":${DBG}}" | jq -rS '.'
else
  if [ ! -z "${DBG}" ]
  then
    echo "curl -s -XPOST -H Content-Type: application/json -H Authorization: Bearer ${TOKEN} https://api-gw.dev.platform.linuxfoundation.org/insights-service/v2/connectors/rocketchat -d{\"rocketchat_url\":\"${1}\",\"channel\":\"${2}\",\"rocketchat_debug\":${DBG}} | jq -rS ."
  fi
  curl -s -XPOST -H 'Content-Type: application/json' -H "Authorization: Bearer ${TOKEN}" 'https://api-gw.dev.platform.linuxfoundation.org/insights-service/v2/connectors/rocketchat' -d"{\"rocketchat_url\":\"${1}\",\"channel\":\"${2}\",\"rocketchat_debug\":${DBG}}" | jq -rS '.'
fi
