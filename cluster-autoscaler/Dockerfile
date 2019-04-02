FROM ubuntu:16.04
MAINTAINER guohaowang
RUN apt-get update -y
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y curl iproute2 inetutils-tools telnet inetutils-ping
COPY cluster-autoscaler  /cluster-autoscaler
RUN chmod a+x /cluster-autoscaler
