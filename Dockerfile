FROM alpine:latest
RUN mkdir /octopus
RUN apk add curl --no-cache 
RUN curl -L https://github.com/OctopusDeploy/cli/raw/main/scripts/install.sh | INSTALL_PATH=/octopus sh

WORKDIR /octopus
ENTRYPOINT ["/octopus/octopus"]

