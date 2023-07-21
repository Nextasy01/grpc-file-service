package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"github.com/Nextasy01/grpc-file-service/client"
	"github.com/Nextasy01/grpc-file-service/pb"
	"github.com/Nextasy01/grpc-file-service/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	username = "admin"
	password = "secret"
)

const (
	username1       = "admin1"
	password1       = "secret"
	refreshDuration = 30 * time.Second
)

func authMethods() map[string]bool {
	const fileServicePath = "/file.service.FileService/"

	return map[string]bool{
		fileServicePath + "Upload":   true,
		fileServicePath + "Download": true,
		fileServicePath + "List":     true,
	}
}

func main() {
	serverAddress := flag.String("address", "", "the server address")

	fileToUploadPath := flag.String("u", "", "file path in your system")
	fileToDownloadId := flag.String("d", "", "id of the file to download")
	numOfConcurrentRequests := flag.Int("num", 1, "number of concurrent request for upload/download")
	fileOption := flag.String("option", "list", "upload, list, download")
	clientNum := flag.String("test", "1", "for testing")
	flag.Parse()

	log.Printf("connecting to server %s", *serverAddress)

	cc1, err := grpc.Dial(*serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("cannot connect to server: ", err)
	}

	var authClient *client.AuthClient

	if *clientNum == "1" {
		authClient = client.NewAuthClient(cc1, username, password)
	} else {
		authClient = client.NewAuthClient(cc1, username1, password1)
	}

	interceptor, err := client.NewAuthInterceptor(authClient, authMethods(), refreshDuration)
	if err != nil {
		log.Fatal("cannot create auth interceptor: ", err)
	}

	cc2, err := grpc.Dial(
		*serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(interceptor.Unary()),
		grpc.WithStreamInterceptor(interceptor.Stream()))
	if err != nil {
		log.Fatal("cannot connect to server: ", err)
	}

	fileClient := service.NewFileClient(cc2)
	if *clientNum == "1" {
		switch *fileOption {
		case "upload":
			testUploadFile(fileClient, username, *fileToUploadPath, *numOfConcurrentRequests)
		case "list":
			testListFiles(fileClient, username)
		case "download":
			testDownloadFile(fileClient, *fileToDownloadId, *numOfConcurrentRequests)
		default:
			log.Fatal("Invalid option")
		}

	} else { // in case you need one more client or more
		switch *fileOption {
		case "upload":
			testUploadFile(fileClient, username1, *fileToUploadPath, *numOfConcurrentRequests)
		case "list":
			testListFiles(fileClient, username1)
		case "download":
			testDownloadFile(fileClient, *fileToDownloadId, *numOfConcurrentRequests)
		default:
			log.Fatal("Invalid option")
		}
	}

}

func testDownloadFile(fc *service.FileClient, path string, num int) {
	var wg sync.WaitGroup

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(i int) {
			fc.Download(path)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func testUploadFile(fc *service.FileClient, name, path string, num int) {
	var wg sync.WaitGroup

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(i int) {
			fc.UploadFile(&pb.Owner{Name: name}, path)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func testListFiles(fc *service.FileClient, name string) {
	fc.ListFiles(&pb.Owner{Name: name})
}
