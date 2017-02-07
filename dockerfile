FROM ubuntu:latest
MAINTAINER Brecht Van Maldergem "brecht.vanmaldergem@gmail.com"

#WORKDIR /home/root
RUN pwd
RUN apt-get update && apt-get upgrade -y && apt-get install -y \
        golang 
COPY slack_daemon.go /home/root/slack_daemon.go
RUN go run slack_daemon.go

