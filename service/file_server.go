package service

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/Nextasy01/grpc-file-service/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxFileSize = 1 << 30 // gigabyte
const maxChunkSize = 1024

const (
	uploadLimit   = 10
	downloadLimit = 10
	listLimit     = 100
)

// File server that provides file services such as upload, list, etc
type FileServer struct {
	pb.UnimplementedFileServiceServer
	fileStore    FileStore
	requestCount atomic.Int32
}

func NewFileServer(fileStore FileStore) *FileServer {
	return &FileServer{fileStore: fileStore}
}

// Return list of uploaded files of client
func (server *FileServer) List(req *pb.ListFilesRequest, stream pb.FileService_ListServer) error {
	server.requestCount.Add(1) // incrementing concurent request count
	defer server.requestCount.Add(-1)

	for {
		if server.requestCount.Load() > listLimit {
			log.Printf("List limit(%d) is exceeded. Waiting for other files finish showing", listLimit)
		} else {
			break
		}
	}
	log.Println("Returning list of uploaded files for user: ", req.GetUser().GetName())

	files := server.fileStore.List(req.GetUser().GetName())

	for _, file := range files {
		err := contextError(stream.Context())
		if err != nil {
			return err
		}

		res := &pb.ListFilesResponse{
			File: file,
		}

		err = stream.Send(res)
		if err != nil {
			return status.Errorf(codes.Internal,
				"failed to send response: %v", err)
		}
	}

	log.Println("Total files returned:", len(files))
	return nil
}

// Uploads a file from client to server
func (server *FileServer) Upload(stream pb.FileService_UploadServer) error {
	server.requestCount.Add(1) // incrementing concurent request count
	defer server.requestCount.Add(-1)

	for {
		if server.requestCount.Load() > uploadLimit {
			log.Printf("Upload limit(%d) is exceeded. Waiting for other uploads to finish", uploadLimit)
		} else {
			break
		}
	}
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	fullName := req.GetFile().GetTitle()

	log.Printf("Received request to upload file - %s", fullName)

	file := NewFile()

	var fileSize uint32
	fileSize = 0.0

	defer file.Output.Close()

	for {
		err := contextError(stream.Context())
		if err != nil {
			return err
		}

		log.Println("Waiting for data to receive")

		req, err := stream.Recv()
		if err == io.EOF {
			log.Println("No more data to receive")
			break
		}

		if err != nil {
			log.Println("Cannot receive a chunk of data", err)
			return err
		}

		chunk := req.GetChunk()
		fileSize += uint32(len(chunk))

		log.Printf("Receive a chunk with size: %d", len(chunk))

		if fileSize > maxFileSize {
			log.Println("The file size is too large")
			return status.Errorf(codes.InvalidArgument,
				"the file size is too large. Expected < %d bytes", maxFileSize)
		}

		_, err = file.buffer.Write(chunk)
		if err != nil {
			log.Println("Cannot write a chunk of data", err)
			return err
		}
	}

	err = server.fileStore.Save(req.GetFile(), *file.buffer)
	if err != nil {
		log.Println("Cannot save file to the store ", err)
		return err
	}

	res := &pb.UploadFileResponse{
		File: req.File,
		Size: fileSize,
	}

	err = stream.SendAndClose(res)
	if err != nil {
		log.Println("Cannot send response ", err)
		return err
	}

	log.Printf("Saved file - %s with size %d bytes", filepath.Base(fullName), fileSize)

	return nil
}

// downloads file from server and returns it to client
func (server *FileServer) Download(req *pb.DownloadFileRequest, stream pb.FileService_DownloadServer) error {
	server.requestCount.Add(1) // incrementing concurent request count
	defer server.requestCount.Add(-1)

	for {
		if server.requestCount.Load() > downloadLimit {
			log.Printf("Download limit(%d) is exceeded. Waiting for other files finish downloading", downloadLimit)
		} else {
			break
		}
	}

	if req.GetFileId() == "" {
		return status.Error(codes.InvalidArgument, "filename is required")
	}

	file := server.fileStore.Find(req.GetFileId())
	if file == nil {
		return status.Errorf(codes.NotFound, "file with id \"%s\" was not found", req.GetFileId())
	}

	err := stream.SendHeader(Metadata(file)) // we are sending file metadata to headers once
	if err != nil {
		return status.Error(codes.Internal, "couldn't send file metadata")
	}

	fileToSend := NewFile()

	fileToSend.Path = "../files/" + file.GetId() + "." + strings.Split(file.GetTitle(), ".")[1]

	f, err := os.Open(fileToSend.Path)
	if err != nil {
		return err
	}

	res := &pb.DownloadFileResponse{Chunk: make([]byte, maxChunkSize)}
	var n int

	for {
		err := contextError(stream.Context())
		if err != nil {
			return err
		}

		log.Println("Sending data")

		n, err = f.Read(res.Chunk)
		if err == io.EOF {
			log.Println("No more data to send")
			break
		}

		if err != nil {
			log.Println("Cannot read a chunk of data", err)
			return err
		}

		res.Chunk = res.Chunk[:n]
		err = stream.Send(res)
		if err != nil {
			log.Println("Cannot send a chunk of data", err)
			return status.Errorf(codes.Internal, "server.Send: %v", err)
		}

	}

	return nil
}

// the numerous cases of context error
func contextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return status.Error(codes.Canceled, "request is canceled")
	case context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, "deadline is exceeded")
	default:
		return nil
	}
}
