package service

import (
	"strconv"

	"github.com/Nextasy01/grpc-file-service/pb"
	"google.golang.org/grpc/metadata"
)

func Metadata(file *pb.File) metadata.MD {
	return metadata.New(map[string]string{
		"ID":    file.GetId(),
		"Title": file.GetTitle(),
		"Size":  strconv.Itoa(int(file.GetSize())),
	})
}
