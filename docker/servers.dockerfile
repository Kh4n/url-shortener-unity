FROM golang:buster

COPY . /url-shortener-unity

RUN mkdir /url-shortener-unity/bin
RUN cd /url-shortener-unity && go test ./...
RUN cd /url-shortener-unity && go build -o ./bin/dbserver ./go/servers/db/main.go
RUN cd /url-shortener-unity && go build -o ./bin/cacheserver ./go/servers/cache/main.go
RUN cd /url-shortener-unity && go build -o ./bin/webappserver ./go/servers/webapp/main.go