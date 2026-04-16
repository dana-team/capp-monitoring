FROM registry.access.redhat.com/ubi9/ubi-minimal

RUN microdnf install -y \
    wget tar gzip curl jq bash \
    && microdnf clean all

WORKDIR /tmp/build

# hey
RUN curl -sSL -o /usr/local/bin/hey \
    https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_amd64 \
    && chmod +x /usr/local/bin/hey

# k6 v0.52.0
RUN curl -sSL -o k6.tgz \
    https://github.com/grafana/k6/releases/download/v0.52.0/k6-v0.52.0-linux-amd64.tar.gz \
    && tar -xzf k6.tgz \
    && mv k6-v0.52.0-linux-amd64/k6 /usr/local/bin/k6 \
    && chmod +x /usr/local/bin/k6

# vegeta v12.13.0
RUN curl -sSL -o vegeta.tgz \
    https://github.com/tsenart/vegeta/releases/download/v12.13.0/vegeta_12.13.0_linux_amd64.tar.gz \
    && tar -xzf vegeta.tgz \
    && mv vegeta /usr/local/bin/vegeta \
    && chmod +x /usr/local/bin/vegeta

# iter8 CLI
RUN curl -sSL https://iter8.tools/install.sh | ITER8_VERSION=0.17.3 bash \
    && mv iter8 /usr/local/bin/iter8

# kubectl (for cold-start scale-to-zero)
RUN curl -sSL -o /usr/local/bin/kubectl \
    "https://dl.k8s.io/release/v1.29.0/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

RUN rm -rf /tmp/build

WORKDIR /scripts
CMD ["/bin/bash"]
