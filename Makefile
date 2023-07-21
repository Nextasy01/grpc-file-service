gen:
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative --go-grpc_out=pb --go-grpc_opt=paths=source_relative proto/*.proto

start:
	mkdir files && mkdir temp_files

clean:
	rm pb/*.go

clear:
	rm files/* && rm temp_files/*

server:
	go run cmd/server/main.go -port 9000

client_upload:
	go run cmd/client/main.go -address 0.0.0.0:9000 -test 1 -option upload -u moon.jpg -num 20

client_list:
	go run cmd/client/main.go -address 0.0.0.0:9000 -test 1 -option list

.PHONY: gen clean clear server start