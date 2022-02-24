#!/bin/bash
export AWS_REGION="`cat ./secrets/AWS_REGION.dev.secret`"
export AWS_ACCESS_KEY_ID="`cat ./secrets/AWS_ACCESS_KEY_ID.dev.secret`"
export AWS_SECRET_ACCESS_KEY="`cat ./secrets/AWS_SECRET_ACCESS_KEY.dev.secret`"
export ENCRYPTION_KEY="`cat ./secrets/ENCRYPTION_KEY.dev.secret`"
export ENCRYPTION_BYTES="`cat ./secrets/ENCRYPTION_BYTES.dev.secret`"
export ESURL="`cat ./secrets/ES_URL.prod.secret`"
export STREAM=''
export ROCKETCHAT_NO_INCREMENTAL=1
# curl -s -XPOST -H 'Content-Type: application/json' "${ESURL}/last-update-cache/_delete_by_query" -d'{"query":{"term":{"key.keyword":"RocketChat:https://chat.hyperledger.org quilt"}}}' | jq -rS '.' || exit 1
../insights-datasource-github/encrypt "`cat ./secrets/user.secret`" > ./secrets/user.encrypted.secret || exit 2
../insights-datasource-github/encrypt "`cat ./secrets/token.secret`" > ./secrets/token.encrypted.secret || exit 3
./rocketchat --rocketchat-url='https://chat.hyperledger.org' --rocketchat-channel='supply-chain-sig' --rocketchat-es-url="${ESURL}" --rocketchat-debug=0 --rocketchat-user="`cat ./secrets/user.encrypted.secret`" --rocketchat-token="`cat ./secrets/token.encrypted.secret`" --rocketchat-stream="${STREAM}" $* 2>&1 | tee run.log
#../insights-datasource-github/encrypt "`cat ./secrets/user-ccc.secret`" > ./secrets/user-ccc.encrypted.secret || exit 2
#../insights-datasource-github/encrypt "`cat ./secrets/token-ccc.secret`" > ./secrets/token-ccc.encrypted.secret || exit 3
#./rocketchat --rocketchat-url='https://chat.enarx.dev' --rocketchat-channel='general' --rocketchat-es-url="${ESURL}" --rocketchat-debug=0 --rocketchat-user="`cat ./secrets/user-ccc.encrypted.secret`" --rocketchat-token="`cat ./secrets/token-ccc.encrypted.secret`" --rocketchat-stream="${STREAM}" $* 2>&1 | tee run.log
