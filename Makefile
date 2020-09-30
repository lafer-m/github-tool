
TG="github"

build:
	go build -o $(TG) .

install: build
	cp $(TG) $(GOPATH)/bin
