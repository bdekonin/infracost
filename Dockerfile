FROM golang:1.15 as builder

ARG ARCH=linux
ARG TERRAFORM_VERSION=0.14.2
ARG TERRAGRUNT_VERSION=0.26.7
ARG TERRAFORM_PROVIDER_INFRACOST_VERSION=latest

# Set Environment Variables
SHELL ["/bin/bash", "-c"]
ENV HOME /app
ENV CGO_ENABLED 0

# Install Packages
RUN wget -q https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${ARCH}_amd64.zip
RUN wget -q https://github.com/gruntwork-io/terragrunt/releases/download/v$TERRAGRUNT_VERSION/terragrunt_linux_amd64
RUN apt-get update -q && apt-get -y install zip jq -y
RUN unzip terraform*.zip && \
    mv terraform /usr/bin && \
    chmod +x /usr/bin/terraform
RUN mv terragrunt_linux_amd64 /usr/bin/terragrunt && \
    chmod +x /usr/bin/terragrunt

WORKDIR /app
COPY scripts/install_provider.sh scripts/install_provider.sh
RUN scripts/install_provider.sh ${TERRAFORM_PROVIDER_INFRACOST_VERSION} /usr/bin/

# Build Application
COPY . .
RUN make deps
RUN NO_DIRTY=true make build

# Application
FROM alpine:3.12 as app
# Tools needed for running diffs in CI integrations
RUN apk --update --no-cache add ca-certificates openssl curl git jq
WORKDIR /root/
# Scripts are used by CI integrations and other use-cases
COPY scripts /scripts
COPY --from=builder /usr/bin/terraform* /usr/bin/
COPY --from=builder /usr/bin/terragrunt /usr/bin/
COPY --from=builder /app/build/infracost /usr/bin/
RUN chmod +x /usr/bin/infracost
ENTRYPOINT [ "infracost" ]
