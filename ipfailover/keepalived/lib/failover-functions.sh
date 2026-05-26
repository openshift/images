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

  # Remove the nftables table we created for keepalived multicast.
  # Unlike iptables where we removed a single rule from a shared chain,
  # nft gives us an isolated table ("inet keepalived") that only we own,
  # so deleting the entire table is the correct cleanup.
  if [[ -n "${HA_MULTICAST_ACCEPT:-}" ]]; then
    if nft list table inet keepalived > /dev/null 2>&1 ; then
      echo "  - Removing keepalived multicast nftables rules ..."
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

  # When HA_MULTICAST_ACCEPT is set, ensure an nftables rule exists to
  # explicitly allow VRRP multicast traffic (224.0.0.18) used by keepalived.
  # The chain policy is "accept" so this rule is defense-in-depth, not
  # load-bearing — traffic would flow without it, but we keep it as an
  # explicit signal that multicast is expected.
  #
  # nft creates an isolated table ("inet keepalived") with its own chain
  # hooked into the input path. This is different from iptables where we
  # inserted a rule into an existing shared chain. The table name is fixed
  # and does not come from the env var value.
  if [[ -n "${HA_MULTICAST_ACCEPT:-}" ]]; then
    echo "  - Ensuring nftables rule for keepalived multicast (224.0.0.18) ..."
    if ! nft list chain inet keepalived filter 2>/dev/null | grep -q 'ip daddr 224.0.0.18 accept' ; then
      echo "  - Adding nftables rule to accept multicast 224.0.0.18."
      # Atomic batch: create the table, chain, and rule in one nft -f call.
      # "inet" family handles both IPv4 and IPv6.
      # Hook "input" at priority 0 matches where the old iptables rule lived.
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

