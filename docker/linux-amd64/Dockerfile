FROM --platform=linux/amd64 alpine:latest 

ARG OCTOPUS_CLI_VERSION=2.4.0

RUN apk update && apk upgrade && apk add --no-cache bash curl

RUN curl -L https://github.com/OctopusDeploy/cli/raw/main/scripts/install.sh | VERSION=v$OCTOPUS_CLI_VERSION bash

LABEL \
    org.label-schema.schema-version="1.0" \
    org.label-schema.name="Octopus Deploy CLI" \
    org.label-schema.vendor="Octopus Deploy" \
    org.label-schema.url="https://octopus.com" \
    org.label-schema.vcs-url="https://github.com/OctopusDeploy/cli" \
    org.label-schema.license="Apache"  \
    org.label-schema.description="Octopus Deploy CLI: octopus" 

ENTRYPOINT ["octopus"]