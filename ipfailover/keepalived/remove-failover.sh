#!/bin/bash

#  Includes.
source "$(dirname "${BASH_SOURCE[0]}")/lib/failover-functions.sh"

echo "`basename $0`: OpenShift IP Failover service terminating."
unconfigure_failover
