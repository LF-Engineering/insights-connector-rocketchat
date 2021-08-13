#!/bin/bash
clear; ROCKETCHAT_TAGS="c,d,e" ./scripts/rocketchat.sh --rocketchat-date-from "2021-01" --rocketchat-date-to "2021-08" --rocketchat-pack-size=10 --rocketchat-tags="a,b,c" --rocketchat-project=Kubernetes --rocketchat-max-items=50 --rocketchat-min-rate=3 --rocketchat-wait-rate --rocketchat-debug=2
