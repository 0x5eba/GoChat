command -v go >/dev/null 2>&1 || { echo >&2 "Install Golang before start"; exit 1; }

go get -u github.com/golang/protobuf/{proto,protoc-gen-go}

echo "Downloading the dependency of the project"
go get -d ./...

go build -o chat