# This file is for mere mortals that do not usually work with Go.

GOBIN = bin
GOBUILD = env CGO_ENABLED=1 go build -o

raido:
	$(GOBUILD) bin/raido cmd/blockchain/main.go
	@echo "Done building raido."
	@echo "Run \"$(GOBIN)/raido\" to launch RaidoChain node."

validator:
	$(GOBUILD) bin/validator cmd/validator/main.go
	@echo "Done building raido validator."
	@echo "Run \"$(GOBIN)/validator\" to launch RaidoChain validator node."

keygen:
	$(GOBUILD) bin/raido-keygen cmd/keygen/main.go
	@echo "Done building keygen."
	@echo "Run \"$(GOBIN)/raido-keygen\" to generate keys."

compile: 
	protoc -I=./proto/ -I=./proto/ext/ -I=./proto/prototype/ \
	--go_out=./proto/ --go-grpc_out=./proto/  --validate_out="lang=go:./proto/" \
	-I=./proto/ext/protoc-gen-validate/ -I=./proto/ext/googleapis/ \
	-I=./proto/ext/grpc-gateway/ \
	--grpc-gateway_out=./proto/ --grpc-gateway_opt=paths=source_relative \
	--go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
	./proto/prototype/*.proto 

all:
		raido
		keygen