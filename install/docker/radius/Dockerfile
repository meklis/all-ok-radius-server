FROM ubuntu:18.04
LABEL maintainer="Max Boyar <max.boyar.a@gmail.com>"
RUN apt update && apt -y upgrade
ADD https://github.com/meklis/all-ok-radius-server/releases/download/0.0.1/all-ok-radius-server-linux-amd-64 /opt/all-ok-radius-server
COPY radius.server.conf.yml /opt/radius.server.conf.yml
RUN chmod +x /opt/all-ok-radius-server
ENTRYPOINT ["/opt/all-ok-radius-server", "-c", "/opt/radius.server.conf.yml"]

