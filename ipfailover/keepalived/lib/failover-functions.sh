#!/bin/bash


#  Includes.
mydir=$(dirname "${BASH_SOURCE[0]}")
source "$mydir/../conf/settings.sh"
source "$mydir/utils.sh"
source "$mydir/config-generators.sh"

#  Constants.
readonly KEEPALIVED_CONFIG=${KEEPALIVED_CONFIG:-"/etc/keepalived/keepalived.conf"}
readonly KEEPALIVED_DEFAULTS="/etc/sysconfig/keepalived"


function cleanup() {
  echo "  - Cleaning up ... "
  [ -n "$1" ] && kill -TERM $1

  local interface=$(get_network_device "$NETWORK_INTERFACE")
  local vips=$(expand_ip_ranges "$HA_VIPS")
  echo "  - Releasing VIPs ${vips} (interface ${interface}) ... "

  local regex='^.*?/[0-9]+$'

  for vip in ${vips}; do
    echo "  - Releasing VIP ${vip} ... "
    if [[ ${vip} =~ ${regex} ]] ; then
      ip addr del ${vip} dev ${interface} || :
    else
      ip addr del ${vip}/32 dev ${interface} || :
    fi
  done

  exit 0
}

function unconfigure_failover() {
  echo "  - Removing ip_vs module ..."
  modprobe -r ip_vs

  if [[ -n "${HA_IPTABLES_CHAIN:-}" ]]; then
    if nft list table inet keepalived > /dev/null 2>&1 ; then
      echo "  - Removing keepalived multicast nft rules ..."
      nft delete table inet keepalived
    fi
  fi

  cleanup $(pidof /usr/sbin/keepalived)
}

function setup_failover() {
  echo "  - Loading ip_vs module ..."
  modprobe ip_vs

  echo "  - Checking if ip_vs module is available ..."
  if lsmod | grep '^ip_vs'; then
    echo "  - Module ip_vs is loaded."
  else
    echo "ERROR: Module ip_vs is NOT available."
  fi

  if [[ -n "${HA_IPTABLES_CHAIN:-}" ]]; then
    echo "  - Ensuring nft rule for keepalived multicast (224.0.0.18) ..."
    if ! nft list chain inet keepalived filter 2>/dev/null | grep -q 'ip daddr 224.0.0.18 accept' ; then
      echo "  - Adding nft rule to accept multicast 224.0.0.18."
      nft -f - <<-'NFT'
	table inet keepalived {
	  chain filter {
	    type filter hook input priority 0; policy accept;
	    ip daddr 224.0.0.18 accept
	  }
	}
	NFT
    fi
  fi

  echo "  - Generating and writing config to $KEEPALIVED_CONFIG"
  generate_failover_config > "$KEEPALIVED_CONFIG"
}


function start_failover_services() {
  echo "  - Starting failover services ..."

  [ -f "$KEEPALIVED_DEFAULTS" ] && source "$KEEPALIVED_DEFAULTS"

  killall -9 /usr/sbin/keepalived &> /dev/null || :
  /usr/sbin/keepalived $KEEPALIVED_OPTIONS -n --log-console &
  local pid=$!

  trap "cleanup ${pid}" SIGHUP SIGINT SIGTERM
  wait ${pid}
}

