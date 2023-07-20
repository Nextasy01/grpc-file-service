package service

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Nextasy01/grpc-file-service/pb"
	"github.com/google/uuid"
)

// ErrAlreadyExists is returned when a record with the same ID already exists in the store
var ErrAlreadyExists = errors.New("record already exists")

type FileStore interface {
	Save(file *pb.File, data bytes.Buffer) error
	List(username string) []*pb.File
	Find(filename string) *pb.File
}

type InMemoryFileStore struct {
	mutex      sync.RWMutex
	fileFolder string
	data       map[string]*pb.File
}

func NewInMemoryFileStore(dir string) *InMemoryFileStore {
	return &InMemoryFileStore{
		data:       make(map[string]*pb.File),
		fileFolder: dir,
	}
}

func (store *InMemoryFileStore) Save(file *pb.File, data bytes.Buffer) error {

	fileId, _ := uuid.NewRandom()
	fileType := filepath.Ext(file.GetTitle()) //strings.Split(file.GetTitle(), ".")[1]
	filePath := fmt.Sprintf("%s/%s%s", store.fileFolder, fileId, fileType)
	file.Id = fileId.String()
	file.Title = filepath.Base(file.GetTitle())

	newFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer newFile.Close()

	_, err = data.WriteTo(newFile)
	if err != nil {
		return err
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.data[fileId.String()] = file
	return nil
}

func (store *InMemoryFileStore) Find(filename string) *pb.File {
	file, ok := store.data[filename]
	if ok {
		return file
	}
	return nil
}

func (store *InMemoryFileStore) List(username string) []*pb.File {
	files := make([]*pb.File, 0)
	for _, v := range store.data {
		if v.Owner.Name == username {
			files = append(files, v)
		}
	}
	return files
}
