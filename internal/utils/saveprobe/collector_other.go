//go:build !windows

package saveprobe

type Collector struct{}

func Supported() bool {
	return false
}

func Start(gameDirectory string) (*Collector, error) {
	return nil, ErrUnsupported
}

func (c *Collector) Stop() (Result, error) {
	return Result{}, ErrUnsupported
}
