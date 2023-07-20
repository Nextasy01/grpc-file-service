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
	fileToUploadPath := flag.String("upload", "", "file path in your system")
	numOfConcurrentRequests := flag.Int("num", 10, "number of concurrent request for upload/download")
	// fileToDownloadId := flag.String("download", "", "id of the file to download")
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
		testUploadFile(fileClient, username, *fileToUploadPath, *numOfConcurrentRequests)
		testListFiles(fileClient, username)

	} else { // in case you need one more client or more
		testUploadFile(fileClient, username1, *fileToUploadPath, *numOfConcurrentRequests)
		testListFiles(fileClient, username1)
	}

	// testDownloadFile(fileClient, *fileToDownloadId)
}

func testDownloadFile(fc *service.FileClient, path string) {
	fc.Download(path)
}

func testUploadFile(fc *service.FileClient, name, path string, num int) {
	var wg sync.WaitGroup

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(i int) {
			// if i >= 10 {
			// 	time.Sleep(3 * time.Second)
			// }
			fc.UploadFile(&pb.Owner{Name: name}, path)
			wg.Done()
		}(i)
	}
	wg.Wait()

	// fc.UploadFile(&pb.Owner{Name: name}, "E:/temp/books.csv")
	// fc.UploadFile(&pb.Owner{Name: name}, "E:/temp/test.png")
	// fc.UploadFile(&pb.Owner{Name: name}, "E:/temp/resume.pdf")
}

func testListFiles(fc *service.FileClient, name string) {
	fc.ListFiles(&pb.Owner{Name: name})
}
