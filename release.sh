GOOS=linux GOARCH=amd64 go build -o releases/dialtone-watcher-$1-linux-amd64
GOOS=linux GOARCH=arm64 go build -o releases/dialtone-watcher-$1-linux-arm64
go build -o releases/dialtone-watcher-$1-darwin-arm64
