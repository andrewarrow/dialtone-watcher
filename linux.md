 - go test ./... passed on macOS
  - GOOS=linux GOARCH=amd64 go build ./... passed
  - docker build --target test -t dialtone-watcher:test . passed with real Linux go test ./...
  - docker build --target runtime -t dialtone-watcher:linux . passed
  - docker run --rm dialtone-watcher:linux help passed

  Use:

  - docker build --target test -t dialtone-watcher:test .
  - docker build --target runtime -t dialtone-watcher:linux .
  - docker run --rm dialtone-watcher:linux help

  If you want, I can also add a small make linux-test / make docker-test wrapper so this is one command
  instead of three.
