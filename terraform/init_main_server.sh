sudo yum install -y golang memcached ansible git

git clone https://github.com/Kh4n/url-shortener-unity.git
cd url-shortener-unity
mkdir bin
go build -o ./bin/main_server ./go/main-server/main.go
go test ./...
./bin/main_server -port=80