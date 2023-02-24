package images

import (
	"github.com/sirupsen/logrus"
	"os"
)

type Storage struct {
	Logger *logrus.Logger
	Path   string
}

func New(logger *logrus.Logger, path string) (storage Storage, err error) {
	storage.Logger = logger
	logger.Println("initialising images store")

	// attempt to create an images directory if it doesn't exist
	if err = os.MkdirAll(path, 0750); err != nil {
		return storage, err
	}

	// the path has been validated; remember to check for permissions tk
	storage.Path = path

	return storage, nil
}
