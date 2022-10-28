#
# VIP failover monitoring container for OpenShift.
#
# ImageName: openshift/origin-keepalived-ipfailover
#
FROM registry.ci.openshift.org/ocp/4.12:base

RUN INSTALL_PKGS="kmod keepalived iproute psmisc nmap-ncat net-tools ipset ipset-libs procps-ng" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all
COPY . /var/lib/ipfailover/keepalived/

COPY iptables-scripts/iptables /usr/sbin/
COPY iptables-scripts/iptables-save /usr/sbin/
COPY iptables-scripts/iptables-restore /usr/sbin/
COPY iptables-scripts/ip6tables /usr/sbin/
COPY iptables-scripts/ip6tables-save /usr/sbin/
COPY iptables-scripts/ip6tables-restore /usr/sbin/

LABEL io.k8s.display-name="OpenShift IP Failover" \
      io.k8s.description="This is a component of OpenShift and runs a clustered keepalived instance across multiple hosts to allow highly available IP addresses." \
      io.openshift.tags="openshift,ha,ip,failover"
EXPOSE 1985
WORKDIR /var/lib/ipfailover
ENTRYPOINT ["/var/lib/ipfailover/keepalived/monitor.sh"]
