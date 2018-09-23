export GO111MODULE=on
BINARY_NAME=baren

all: deps build
install:
    go install cmd/yourapp/yourapp.go
build:
    go build cmd/yourapp/yourapp.go
test:
    go test -v ./...
clean:
    go clean
    rm -f $(BINARY_NAME)
deps:
    go build -v ./...
upgrade:
    go get -u
