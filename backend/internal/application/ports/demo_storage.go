package ports

import "io"

type DemoStorage interface {
	Save(reader io.Reader, fileName string) (string, error)
}
