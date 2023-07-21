package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Nextasy01/grpc-file-service/pb"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FileClient struct {
	service      pb.FileServiceClient
	requestCount atomic.Int32
}

func NewFileClient(cc *grpc.ClientConn) *FileClient {
	service := pb.NewFileServiceClient(cc)
	return &FileClient{service: service}
}

func (fileClient *FileClient) ListFiles(user *pb.Owner) {
	fileClient.requestCount.Add(1) // incrementing concurent request count
	defer fileClient.requestCount.Add(-1)

	for {
		if fileClient.requestCount.Load() > listLimit {
			log.Printf("List limit(%d) is exceeded. Please wait while other files finish showing", listLimit)
		} else {
			break
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.ListFilesRequest{User: user}

	stream, err := fileClient.service.List(ctx, req)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Your files:")
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Println("Cannot receive response from server: ", err)
			return
		}
		file := res.GetFile()
		fmt.Printf("ID: %s - Name: %s - Date: %s\n", file.GetId(), file.GetTitle(), file.GetCreatedAt().AsTime().UTC())
	}

}

func (fileClient *FileClient) UploadFile(user *pb.Owner, path string) {
	fileClient.requestCount.Add(1) // incrementing concurent request count
	defer fileClient.requestCount.Add(-1)

	for {
		if fileClient.requestCount.Load() > uploadLimit {
			log.Printf("Upload limit(%d) is exceeded. Please wait while other files finish uploading", uploadLimit)
		} else {
			break
		}
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("Wrong path or file doesn't exists in %s path: %v", path, err)
		return
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := fileClient.service.Upload(ctx)
	if err != nil {
		log.Printf("Couldn't upload file, try again: %v", err)
		return
	}

	newId, _ := uuid.NewRandom()
	fileStruct, _ := file.Stat()
	fileSize := fileStruct.Size()

	req := &pb.UploadFileRequest{
		File: &pb.File{
			Id:        newId.String(),
			Title:     file.Name(),
			Size:      float64(fileSize),
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
			Owner:     user,
		},
	}

	err = stream.Send(req)
	if err != nil {
		log.Printf("Couldn't send file to the server, try again later: %v", err)
		return
	}

	reader := bufio.NewReader(file)
	buf := make([]byte, 1024)

	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			log.Println("No more data")
			break
		}

		if err != nil {
			log.Printf("Cannot read a chunk of data to buffer, try again: %v", err)
			return
		}

		req := &pb.UploadFileRequest{Chunk: buf[:n]}

		err = stream.Send(req)
		if err != nil {
			log.Printf("Cannot send a chunk of data to server, try again: %v", err)
			return
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Cannot receive response: %v", err)
		return
	}

	log.Printf("File successfully uploaded with id: %s and size: %d bytes", res.GetFile().GetId(), res.GetSize())
}

func (fileClient *FileClient) Download(id string) {
	fileClient.requestCount.Add(1) // incrementing concurent request count
	defer fileClient.requestCount.Add(-1)

	for {
		if fileClient.requestCount.Load() > downloadLimit {
			log.Printf("Download limit(%d) is exceeded. You can upload only up to 10 files at once, please wait while other files finish downloading", downloadLimit)
		} else {
			break
		}
	}
	log.Println("Starting to download the file")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := fileClient.service.Download(ctx, &pb.DownloadFileRequest{
		FileId: id,
	})
	if err != nil {
		log.Printf("Couldn't download file, try again: %v", err)
		return
	}

	md, err := stream.Header()
	if err != nil {
		log.Printf("Couldn't get file metadata, try again: %v", err)
		return
	}

	r, w := io.Pipe()
	defer r.Close()

	// go routine to receive responses
	go copyFromResponse(w, stream)

	log.Println("Creating new file")

	time.Sleep(500 * time.Millisecond) // this is needed to surely randomize new generated UUID
	randID, _ := uuid.NewUUID()        // since NewUUID based on current time, multiple goroutines may execute at the same time leading on the same UUID

	newFileName := randID.String() + "-" + md.Get("title")[0]
	filePath := filepath.Join("temp_files", newFileName)
	f, err := os.Create(filePath)
	if err != nil {
		log.Printf("Couldn't create file: %v", err)
		return
	}
	defer f.Close()
	log.Println("copying contents from reader pipe to file")
	_, _ = f.ReadFrom(r)

	if err != nil {
		log.Printf("Receive error from response: %v", err)
		return
	}

	log.Printf("Successfully downloaded file with name: %s and size: %s bytes!", newFileName, md.Get("size")[0])

}

func copyFromResponse(w *io.PipeWriter, stream pb.FileService_DownloadClient) {
	var err error
	res := new(pb.DownloadFileResponse)
	for {
		// log.Println("receiving data from server")
		err = stream.RecvMsg(res)
		if err == io.EOF {
			_ = w.Close()
			log.Print("no more responses")
			break
		}
		if err != nil {
			log.Println("cannot receive stream response: ", err)
			return
		}
		if len(res.GetChunk()) > 0 {
			_, err := w.Write(res.Chunk)
			if err != nil {
				log.Printf("Couldn't write to a file: %v", err)
				_ = stream.CloseSend()
				break
			}
		}
		res.Chunk = res.Chunk[:0]
	}
}
