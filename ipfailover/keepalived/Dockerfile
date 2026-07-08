# Test extension builder stage (added by ote-migration)
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.22 AS test-extension-builder
RUN mkdir -p /go/src/github.com/openshift/ipfailover/keepalived
WORKDIR /go/src/github.com/openshift/ipfailover/keepalived
COPY . .
RUN cd tests && \
    make build && \
    cd bin && \
    tar -czvf ipfailover-test-extension.tar.gz ipfailover-tests-ext && \
    rm -f ipfailover-tests-ext

#
# VIP failover monitoring container for OpenShift.
#
# ImageName: openshift/origin-keepalived-ipfailover
#
FROM registry.ci.openshift.org/ocp/4.21:base-rhel9

RUN INSTALL_PKGS="kmod keepalived iproute psmisc nmap-ncat net-tools ipset ipset-libs procps-ng nftables" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all
COPY . /var/lib/ipfailover/keepalived/

# Copy test extension binary (added by ote-migration)
COPY --from=test-extension-builder /go/src/github.com/openshift/ipfailover/keepalived/tests/bin/ipfailover-test-extension.tar.gz /usr/bin/

LABEL io.k8s.display-name="OpenShift IP Failover" \
      io.k8s.description="This is a component of OpenShift and runs a clustered keepalived instance across multiple hosts to allow highly available IP addresses." \
      io.openshift.tags="openshift,ha,ip,failover"
EXPOSE 1985
WORKDIR /var/lib/ipfailover
ENTRYPOINT ["/var/lib/ipfailover/keepalived/monitor.sh"]
