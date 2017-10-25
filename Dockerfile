FROM alpine:3.6

ADD kvm-operator-node-controller /usr/bin/kvm-operator-node-controller
ENTRYPOINT ["/usr/bin/kvm-operator-node-controller"]
