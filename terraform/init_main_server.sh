sudo yum install -y golang ansible git

git clone https://github.com/Kh4n/url-shortener-unity.git
cd url-shortener-unity
mkdir bin
go test ./...
go build -o ./bin/main_server ./go/main-server/main.go
sudo ./bin/main_server -port=80