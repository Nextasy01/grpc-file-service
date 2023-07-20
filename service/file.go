package service

import (
	"bytes"
	"os"
)

type File struct {
	Path   string
	buffer *bytes.Buffer
	Output *os.File
}

func NewFile() *File {
	return &File{buffer: &bytes.Buffer{}}
}
