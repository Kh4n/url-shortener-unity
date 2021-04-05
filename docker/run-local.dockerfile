FROM golang:buster

COPY . /url-shortener-unity

RUN mkdir /url-shortener-unity/bin
RUN cd /url-shortener-unity && go test ./...
RUN cd /url-shortener-unity && go build -o ./bin/dbserver ./go/servers/db/main.go
RUN cd /url-shortener-unity && go build -o ./bin/cacheserver ./go/servers/cache/main.go
RUN cd /url-shortener-unity && go build -o ./bin/webappserver ./go/servers/webapp/main.go

CMD /url-shortener-unity/bin/dbserver \ 
    & /url-shortener-unity/bin/cacheserver -memcachedHost=memcached:11211 \
    & /url-shortener-unity/bin/webappserver -webDir=/url-shortener-unity/web