#
# This is the egress router for OpenShift Origin
#
# The standard name for this image is openshift/origin-egress-router

FROM registry.ci.openshift.org/ocp/4.20:base-rhel9

RUN INSTALL_PKGS="iproute iputils iptables-nft" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

ADD egress-router.sh /bin/egress-router.sh

LABEL io.k8s.display-name="OpenShift Origin Egress Router" \
      io.k8s.description="This is a component of OpenShift Origin and contains an egress router." \
      io.openshift.tags="openshift,router,egress"

ENTRYPOINT /bin/egress-router.sh
