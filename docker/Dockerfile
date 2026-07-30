FROM registry.access.redhat.com/ubi9/ubi-minimal

RUN microdnf install -y \
    tar gzip jq bash \
    && microdnf clean all

WORKDIR /tmp/build

# k6 v0.52.0 (load driver for the scale benchmark)
RUN curl -sSL -o k6.tgz \
    https://github.com/grafana/k6/releases/download/v0.52.0/k6-v0.52.0-linux-amd64.tar.gz \
    && tar -xzf k6.tgz \
    && mv k6-v0.52.0-linux-amd64/k6 /usr/local/bin/k6 \
    && chmod +x /usr/local/bin/k6

# kubectl (for pod-count checks during scale-out)
RUN curl -sSL -o /usr/local/bin/kubectl \
    "https://dl.k8s.io/release/v1.29.0/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

RUN rm -rf /tmp/build

WORKDIR /scripts
CMD ["/bin/bash"]
