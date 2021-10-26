ARG ARCH
FROM golang:1.15.10

WORKDIR /placement-policy-scheduler-plugins
COPY . .
ARG ARCH
ARG RELEASE_VERSION
RUN RELEASE_VERSION=${RELEASE_VERSION} make build-scheduler.$ARCH

FROM $ARCH/alpine:3.12

COPY --from=0 /placement-policy-scheduler-plugins/bin/kube-scheduler /bin/kube-scheduler

WORKDIR /bin
CMD ["kube-scheduler"]
