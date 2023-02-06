cli:
	go build -o bin/cf cmd/main.go
	mv ./bin/cf /usr/local/bin/