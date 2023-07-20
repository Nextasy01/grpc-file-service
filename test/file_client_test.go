package service_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nextasy01/grpc-file-service/pb"
	"github.com/Nextasy01/grpc-file-service/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEverything(t *testing.T) {
	t.Parallel()

	uploadFolder := "../files"
	downloadFolder := "../temp_files"

	fileStore := service.NewInMemoryFileStore(uploadFolder)
	userStore := service.NewInMemoryUserStore()
	user := createUser(t, userStore, "testUser", "secret", "admin")
	serverAddress := startTestFileServer(t, fileStore)
	fileClient := newTestFileClient(t, serverAddress)

	filePath := "../moon.jpg"
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	stream, err := fileClient.Upload(context.Background())
	require.NoError(t, err)

	newId, err := uuid.NewRandom()
	require.NoError(t, err)

	newUserId, err := uuid.NewRandom()
	require.NoError(t, err)

	fileStruct, err := file.Stat()
	require.NoError(t, err)

	fileSize := fileStruct.Size()

	req := &pb.UploadFileRequest{
		File: &pb.File{
			Id:        newId.String(),
			Title:     file.Name(),
			Size:      float64(fileSize),
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
			Owner: &pb.Owner{
				Id:       newUserId.ID(),
				Name:     user.Username,
				Password: user.HashedPassword,
			},
		},
	}

	size := uploadProcess(t, stream, req, file)

	res, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.NotZero(t, res.GetFile().GetId())
	require.EqualValues(t, size, res.GetSize())

	savedFilePath := fmt.Sprintf("%s/%s%s", uploadFolder, res.GetFile().GetId(), filepath.Ext(res.GetFile().GetTitle()))
	require.FileExists(t, savedFilePath) // check if file is saved

	fileId := res.GetFile().GetId() // saving to download by id later

	// Since we don't use persistent storage, we are testing list call along with upload at the same time
	t.Run("List Files", func(t *testing.T) {
		newUserId, err := uuid.NewRandom()
		require.NoError(t, err)

		req := &pb.ListFilesRequest{User: &pb.Owner{
			Id:       newUserId.ID(),
			Name:     user.Username,
			Password: user.HashedPassword,
		}}
		stream, err := fileClient.List(context.Background(), req)
		require.NoError(t, err)

		log.Println("Your files:")
		found := 0
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			file := res.GetFile()
			fmt.Printf("ID: %s - Name: %s - Date: %s\n", file.GetId(), file.GetTitle(), file.GetCreatedAt().AsTime().UTC())
			found += 1
		}
		require.Equal(t, 1, found) // since we uploaded only 1 file, it should be equal to 1
	})

	// Download uploaded file
	t.Run("Download File", func(t *testing.T) {

		req := &pb.DownloadFileRequest{FileId: fileId}

		stream, err := fileClient.Download(context.Background(), req)
		require.NoError(t, err)

		md, err := stream.Header()
		require.NoError(t, err)
		require.Equal(t, fileId, md.Get("id")[0]) //check if we are getting the correct file

		r, w := io.Pipe()
		// defer require.NoError(t, r.Close())
		defer r.Close()

		newFilePath := downloadFolder + "/" + md.Get("title")[0]
		// go routine to receive responses
		go copyFromResponse(t, w, stream)

		log.Println("Creating new file")

		f, err := os.Create(newFilePath)
		require.NoError(t, err)
		// defer require.NoError(t, f.Close())
		defer f.Close()

		log.Println("copying contents from reader pipe to file")
		_, err = f.ReadFrom(r)

		require.NoError(t, err)

	})

}

func startTestFileServer(t *testing.T, fileStore service.FileStore) string {
	laptopServer := service.NewFileServer(fileStore)

	grpcServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcServer, laptopServer)

	listener, err := net.Listen("tcp", ":0") // random available port
	require.NoError(t, err)

	go grpcServer.Serve(listener)

	return listener.Addr().String()
}

func newTestFileClient(t *testing.T, serverAddress string) pb.FileServiceClient {
	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return pb.NewFileServiceClient(conn)
}

func createUser(t *testing.T, userStore service.UserStore, username, password, role string) *service.User {
	user, err := service.NewUser(username, password, role)
	require.NoError(t, err)
	require.NoError(t, userStore.Save(user))
	return user
}

func copyFromResponse(t *testing.T, w *io.PipeWriter, stream pb.FileService_DownloadClient) {
	var err error
	res := new(pb.DownloadFileResponse)
	for {
		log.Println("receiving data from server")
		err = stream.RecvMsg(res)
		if err == io.EOF {
			require.NoError(t, w.Close())
			log.Print("no more responses")
			// os.Remove(fileToDelete)
			break
		}
		if err != nil {
			log.Println(err)
		}
		require.NoError(t, err)

		if len(res.GetChunk()) > 0 {
			n, err := w.Write(res.Chunk)
			log.Printf("Wrote %d bytes to a file", n)
			require.NoError(t, err)
		}
		res.Chunk = res.Chunk[:0]
	}
}

func uploadProcess(t *testing.T, stream pb.FileService_UploadClient, req *pb.UploadFileRequest, file *os.File) int {
	err := stream.Send(req)
	require.NoError(t, err)

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)
	size := 0

	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}

		require.NoError(t, err)
		size += n

		req := &pb.UploadFileRequest{
			Chunk: buffer[:n],
		}

		err = stream.Send(req)
		require.NoError(t, err)
	}

	return size
}
