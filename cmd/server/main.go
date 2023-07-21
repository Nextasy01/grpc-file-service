package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Nextasy01/grpc-file-service/pb"
	"github.com/Nextasy01/grpc-file-service/service"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

func accessibleMethods() map[string][]string {
	const fileServicePath = "/file.service.FileService/"

	return map[string][]string{
		fileServicePath + "Upload":   {"admin"},
		fileServicePath + "Download": {"admin"},
		fileServicePath + "List":     {"admin"},
	}
}

func seedUsers(userStore service.UserStore) error {
	err := createUser(userStore, "admin", "secret", "admin")
	if err != nil {
		return err
	}
	return createUser(userStore, "admin1", "secret", "admin")
}

func createUser(userStore service.UserStore, username, password, role string) error {
	user, err := service.NewUser(username, password, role)
	if err != nil {
		return err
	}

	return userStore.Save(user)
}

func main() {
	port := flag.Int("port", 8080, "server port")
	flag.Parse()

	address := fmt.Sprintf("0.0.0.0:%d", *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("cannot run the server: ", err)
	}

	fileStore := service.NewInMemoryFileStore("files")
	fileServer := service.NewFileServer(fileStore)

	userStore := service.NewInMemoryUserStore()
	err = seedUsers(userStore)
	if err != nil {
		log.Fatal("cannot seed users")
	}

	jwtManager := service.NewJWTManager(os.Getenv("Secret_Key"), 15*time.Minute)
	authServer := service.NewAuthServer(userStore, jwtManager)
	authInterceptor := service.NewAuthInterceptor(jwtManager, accessibleMethods())

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authInterceptor.Unary()),
		grpc.ChainStreamInterceptor(authInterceptor.Stream()),
	)
	pb.RegisterAuthServiceServer(grpcServer, authServer)
	pb.RegisterFileServiceServer(grpcServer, fileServer)
	reflection.Register(grpcServer)

	log.Printf("Start GRPC server at %s", listener.Addr().String())

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal("cannot run grpc server: ", err)
	}

}
