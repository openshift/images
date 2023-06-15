#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM registry.ci.openshift.org/ocp/builder:rhel-9-base-openshift-4.13

COPY *.repo /etc/yum.repos.d/
RUN INSTALL_PKGS=" \
      which tar wget hostname sysvinit-tools util-linux \
      socat tree findutils lsof bind-utils shadow-utils \
      iproute \
      " && \
    echo 'skip_missing_names_on_install=0' >> /etc/yum.conf && \
    yum install -y ${INSTALL_PKGS} && \
    yum clean all && \
    mkdir -p /var/lib/origin

LABEL io.k8s.display-name="OpenShift Origin CentOS 7 Base" \
      io.k8s.description="This is the base image from which all OpenShift Origin images inherit." \
      io.openshift.tags="openshift,base"
