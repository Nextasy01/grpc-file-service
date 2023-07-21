# GRPC FILE SERVER

## Usage
On Linux execute following commands:
```
make start
make server
make client_upload
make client_list
```
To clear uploaded/downloaded files:
```
make clear
```
To download a file from server, you need to execute `make list` first and grab the ID of the file and paste it like here:
```
go run cmd/client/main.go -address 0.0.0.0:9000 -test 1 -option download -d c4cb04aa-30ca-4660-965e-b8368661ef40 -num 10
```
Likewise if you need to manually run the command for upload:
```
go run cmd/client/main.go -address 0.0.0.0:9000 -test 1 -option upload -u moon.jpg -num 10
```
Or list files:
```
go run cmd/client/main.go -address 0.0.0.0:9000 -test 1 -option list
```
Note that `-num` argument is the number of concurrent requests to upload/download. 
`-test` argument is to switch between clients.

On **Windows** just execute commands manually like above.
To run server manually:
```
go run cmd/server/main.go -port 9000
```