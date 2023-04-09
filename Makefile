
run-server:
	go run ./cmd/server

run-client:
	go run ./cmd/client

build-server:
	go build -o server.exe ./cmd/server

build-client:
	go build -o client.exe ./cmd/client
