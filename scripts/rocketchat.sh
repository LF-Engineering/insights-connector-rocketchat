#!/bin/bash
# ESENV=prod|test
if [ -z "${ESENV}" ]
then
  ESENV=test
fi
./rocketchat --rocketchat-url='https://chat.hyperledger.org' --rocketchat-channel='sawtooth' --rocketchat-es-url="`cat ./secrets/ES_URL.${ESENV}.secret`" --rocketchat-user="`cat ./secrets/user.secret`" --rocketchat-token="`cat ./secrets/token.secret`" $*
