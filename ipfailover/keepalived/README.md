### keepalived-ipfailover images

The keepalived-ipfailover images are not available as part of a cluster-bot PR build.
The instructions for ipfailover explicitly tell us to include
the image in the deployment file, but the image will not necessarily
be available as it is listed in the 
[OpenShift ipfailover documentation](https://docs.openshift.com/container-platform/4.17/networking/configuring-ipfailover.html#nw-ipfailover-configuration_configuring-ipfailover) 

As of release 4.17, the documentation listed the image incorrectly as:
```
quay.io/openshift/origin-keepalived-ipfailover
```
As of release 4.18, the image will be correctly listed as:
```
registry.redhat.io/openshift4/ose-keepalived-ipfailover-rhel9:v[productversion4.15+]
```
or
```
registry.redhat.io/openshift4/ose-keepalived-ipfailover:[productversion-upto4.14]
```
depending on version.  For example, for OpenShift version 4.18, it should be:
`registry.redhat.io/openshift4/ose-keepalived-ipfailover-rhel9:v4.18`.

### testing keepalived-ipfailover image changes

The images are not updated for each commit, only for each merge commit.  So in
order to verify a bug fix prior to merge, you need to:

1. Run `/test images` via comment on the PR in gitHub
2. This launches a job whose build log can be accessed via the link
https://prow.ci.openshift.org/view/gs/test-platform-results/pr-logs/pull/openshift_images/<PR number>/pull-ci-openshift-images-master-images
3. In the build log, there is a logged reference to a build farm cluster, e.g. build09,
and a namespace in which your image has been built:
```
   INFO[2024-11-12T19:07:59Z] Using namespace https://console-openshift-console.apps.build09.ci.devcluster.openshift.com/k8s/cluster/projects/ci-op-sblxpin3
```
4. Click on the console link and authenticate with the build cluster.  Once logged in,
copy your login command (click on upper right, your username). In a terminal, do the following:
```
# Paste login comand
$ oc login --token=sha256~....

# Authenticate with build09's internal registry
$ oc registry login

# Enjoy your image
$ oc image info registry.build09.ci.openshift.org/ci-op-sblxpin3/pipeline:keepalived-ipfailover
```
5. This namespace / image will be around for a few hours before it is garbage collected. 
You can mirror it somewhere for testing (e.g. your personal quay repository).
```
$ oc image mirror registry.<build09>.ci.openshift.org/<ci-op-sblxpin3>/pipeline:keepalived-ipfailover quay.io/<username>/keepalived-ipfailover:latest
```
6. Now you can add `quay.io/<username>/keepalived-ipfailover:latest` to your ipfailover deployment config, instead of the
release image listed in the
[OpenShift ipfailover documentation](https://docs.openshift.com/container-platform/4.17/networking/configuring-ipfailover.html#nw-ipfailover-configuration_configuring-ipfailover).

### post-merge keepalived-ipfailover image changes

Post-merge keepalived-ipfailover are listed in the Red Hat registry catalog: https://catalog.redhat.com/search?gs&q=ipfailover.
Be sure to choose the correct OpenShift release.