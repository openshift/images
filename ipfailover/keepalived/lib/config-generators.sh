#!/bin/bash

#  TODO: This follows the initial demo pieces and uses a bash script to
#        generate the keepalived config - rework this into a template
#        similar to how it is done for the haproxy configuration.

#  Includes.
source "$(dirname "${BASH_SOURCE[0]}")/utils.sh"


# Constants.
readonly CHECK_SCRIPT_NAME="chk_${HA_CONFIG_NAME//-/_}"
readonly CHECK_INTERVAL_SECS="${HA_CHECK_INTERVAL}"
readonly VRRP_SLAVE_PRIORITY=42

readonly DEFAULT_PREEMPTION_STRATEGY="preempt_delay 300"
readonly NO_PREEMPTION_STRATEGY="nopreempt"


#
#  Generate global config section.
#
#  Example:
#     generate_global_config  arparp
#
function generate_global_config() {
  local routername ; routername=$(scrub "$1")

  echo "global_defs {"
  echo "   max_auto_priority    99"
  echo "   notification_email {"

  for email in ${ADMIN_EMAILS[@]}; do
    echo "     $email"
  done

  echo "   }"
  echo ""
  echo "   notification_email_from ${EMAIL_FROM:-"ipfailover@openshift.local"}"
  echo "   smtp_server ${SMTP_SERVER:-"127.0.0.1"}"
  echo "   smtp_connect_timeout ${SMTP_CONNECT_TIMEOUT:-"30"}"
  echo "   router_id ${routername}"
  echo "}"
}


#
#  Generate VRRP checker script configuration section.
#    When a check script is provided use it instead of default script
#    The default script is suppressed When port is 0
#
#  Example:
#      generate_script_config
#      generate_script_config "10.1.2.3" 8080
#
function generate_script_config() {
  local serviceip ; serviceip=${1:-"127.0.0.1"}
  local port=${2:-80}

  echo ""
  echo "vrrp_script ${CHECK_SCRIPT_NAME} {"

  if [[ -n "${HA_CHECK_SCRIPT}" ]]; then
    echo "   script \"${HA_CHECK_SCRIPT}\""
  else
    if [[ "${port}" == "0" ]]; then
      echo "   script \"true\""
    else
      echo "   script \"/bin/bash -c '</dev/tcp/${serviceip}/${port}'\""
    fi
  fi

  echo "   interval ${CHECK_INTERVAL_SECS}"
  echo "}"
}


#
#  Generate authentication information section.
#
#  Example:
#      generate_authentication_info
#
function generate_authentication_info() {
  local creds=${1:-"R0ut3r"}
  echo ""
  echo "   authentication {"
  echo "      auth_type PASS"
  echo "      auth_pass ${creds}"
  echo "   }"
}


#
#  Generate track script section.
#
#  Example:
#      generate_track_script
#
function generate_track_script() {
  echo ""
  echo "   track_script {"
  echo "      ${CHECK_SCRIPT_NAME}"
  echo "   }"
}


#
#  Generate multicast + unicast options section based on the values of the
#  MULTICAST_SOURCE_IPADDRESS, UNICAST_SOURCE_IPADDRESS and UNICAST_PEERS
#  environment variables.
#
#  Examples:
#      generate_mucast_options
#
#      UNICAST_SOURCE_IPADDRESS=10.1.1.1 UNICAST_PEERS="10.1.1.2,10.1.1.3" \
#          generate_mucast_options
#
function generate_mucast_options() {
  echo ""

  if [[ -n "${MULTICAST_SOURCE_IPADDRESS}" ]]; then
    echo "    mcast_src_ip ${MULTICAST_SOURCE_IPADDRESS}"
  fi

  if [[ -n "${UNICAST_SOURCE_IPADDRESS}" ]]; then
    echo "    unicast_src_ip ${UNICAST_SOURCE_IPADDRESS}"
  fi

  if [[ -n "${UNICAST_PEERS}" ]]; then
    echo ""
    echo "    unicast_peer {"

    OLD_IFS=$IFS
    IFS=","
    for ip in ${UNICAST_PEERS}; do
      echo "        ${ip}"
    done
    IFS=$OLD_IFS

    echo "    }"
  fi
}



#
#  Generate virtual ip address section.
#
#  Examples:
#      generate_vip_section "10.245.2.3" "enp0s8"
#
#      generate_vip_section "10.1.1.1 10.1.2.2" "enp0s8"
#
#      generate_vip_section "10.42.42.42-45, 10.9.1.1"
#
function generate_vip_section() {
  local interface ; interface=${2:-"$(get_network_device)"}
  echo ""
  echo "   virtual_ipaddress {"

  for ip in ${1}; do
    echo "    ${ip} dev ${interface}"
  done

  echo "   }"
}


#
#  Generate vrrpd instance configuration section.
#
#  Examples:
#      generate_vrrpd_instance_config arp 1 "10.1.2.3" enp0s8 "252" "master" "preempt_delay 300"
#
#      generate_vrrpd_instance_config arp 1 "10.1.2.3" enp0s8 "3" "slave" "nopreempt"
#
#      generate_vrrpd_instance_config ipf-1 4 "10.1.2.3-4" enp0s8 "7"
#
function generate_vrrpd_instance_config() {
  local servicename=$1
  local iid=${2:-"1"}
  local vips=$3
  local interface=$4
  local priority=${5:-"10"}
  local instancetype=${6:-"slave"}
  local preempt=${7:-"${DEFAULT_PREEMPTION_STRATEGY}"}

  local vipname ; vipname=$(scrub "$1")
  local initialstate=""
  local vrrpidoffset=${HA_VRRP_ID_OFFSET:-10}
  local excludedvrrpids=( $HA_EXCLUDED_VRRP_IDS )

  new_vrrp_id=$((vrrpidoffset + iid))
  while [[ " ${excludedvrrpids[*]} " =~ " ${new_vrrp_id} " ]]; do
    ((vrrpidoffset++))
    new_vrrp_id=$((vrrpidoffset + iid))
    HA_VRRP_ID_OFFSET=$vrrpidoffset
  done

  [ "${instancetype}" = "master" ] && initialstate="state MASTER"

  local instance_name ; instance_name=$(generate_vrrp_instance_name "${servicename}" "${iid}")

  local auth_section ; auth_section=$(generate_authentication_info "${servicename}")
  local vip_section ; vip_section=$(generate_vip_section "${vips}" "${interface}")
  # Emit instance
  echo "
vrrp_instance ${instance_name} {
   interface ${interface}
   ${initialstate}
   virtual_router_id ${new_vrrp_id}
   priority ${priority}
   ${preempt}
   ${auth_section}
   $(generate_track_script)
   "
  if [[ -n $HA_NOTIFY_SCRIPT ]]; then
      echo "   notify \"${HA_NOTIFY_SCRIPT}\""
  fi
  echo " $(generate_mucast_options)
   ${vip_section}
}
"

}


#
#  Generate failover configuration.
#
#  Examples:
#      generate_failover_configuration
#
function generate_failover_config() {
  local vips ; vips=$(expand_ip_ranges "${HA_VIPS}")
  local vip_groups ; vip_groups="${HA_VIP_GROUPS}"
  local interface ; interface=$(get_network_device "${NETWORK_INTERFACE}")
  local ipaddr ; ipaddr=$(get_device_ip_address "${interface}")
  local port="${HA_MONITOR_PORT//[^0-9]/}"
  local preempt=${PREEMPTION:-"${DEFAULT_PREEMPTION_STRATEGY}"}

  echo "! Configuration File for keepalived

$(generate_global_config "${HA_CONFIG_NAME}")
$(generate_script_config "${ipaddr}" "${port}")
"

  local ipkey ; ipkey=$(echo "${ipaddr}" | cut -f 4 -d '.')
  local ipslot=$((ipkey % 128))

  local nodecount
  if [[ "${HA_REPLICA_COUNT}" -gt 0 ]]; then
      nodecount="${HA_REPLICA_COUNT}"
  else
      nodecount="1"
  fi

  local idx=$((ipslot % nodecount))
  idx=$((idx + 1))

  local counter=1
  local previous="none"
  local vip_counter=0
  local total_vips=( $vips )
  local vips_per_group=1
  local vips_mod=0

  if [[ $vip_groups -gt 0 ]]; then
    vips_per_group=$((${#total_vips[@]} / vip_groups))
    vips_mod=$((${#total_vips[@]} % vip_groups))
  fi

  while [[ "${vip_counter}" -lt "${#total_vips[@]}" ]]; do
    local cur_vip_count=vips_per_group

    if [[ ${vips_mod} -gt 0 ]]; then
      ((cur_vip_count++))
      ((vips_mod--))
    fi

    vip_group=("${total_vips[@]:vip_counter:cur_vip_count}")
    vip_counter=$((vip_counter + cur_vip_count))

    local offset=$((RANDOM % 32))
    local priority=$((ipslot % 64 + offset))
    local instancetype="slave"
    local n=$((counter % idx))

    if [[ ${n} -eq 0 ]]; then
      # Only set instancetype as master if the preempt strategy is not nopreempt.
      # The nopreempt option doesn't work for VRRPD instances that are masters.
      if [[ "${preempt}" != "${NO_PREEMPTION_STRATEGY}" ]]; then
        instancetype="master"
      fi
      if [[ "${previous}" == "master" ]]; then
        #  Inverse priority + reset, so that we can flip-flop priorities.
        priority=$((ipslot + 1))
        previous="flip-flop"
      else
        priority=$((255 - ipslot))
        previous=${instancetype}
      fi
    fi

    generate_vrrpd_instance_config "${HA_CONFIG_NAME}" "${counter}" "${vip_group[*]}" \
      "${interface}" "${priority}" "${instancetype}" "${preempt}"
    ((counter++))

  done
}

