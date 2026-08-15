package httputils

import "io"

// TransferProgress describes the current byte offset of a streamed transfer.
type TransferProgress struct {
	Current int64
	Total   int64
}

// ProgressCallback receives synchronous transfer progress updates.
type ProgressCallback func(TransferProgress)

// ProgressReadSeeker reports read progress while retaining seek support for
// clients that rewind a request body before retrying it.
type ProgressReadSeeker struct {
	source   io.ReadSeeker
	total    int64
	current  int64
	callback ProgressCallback
}

// NewProgressReadSeeker wraps source with byte progress reporting. A nil
// source produces a nil result. Negative totals are reported as zero.
func NewProgressReadSeeker(source io.ReadSeeker, total int64, callback ProgressCallback) io.ReadSeeker {
	if source == nil {
		return nil
	}
	if total < 0 {
		total = 0
	}
	return &ProgressReadSeeker{
		source:   source,
		total:    total,
		callback: callback,
	}
}

func (r *ProgressReadSeeker) Read(buffer []byte) (int, error) {
	read, err := r.source.Read(buffer)
	if read > 0 {
		r.current += int64(read)
		r.report()
	}
	return read, err
}

func (r *ProgressReadSeeker) Seek(offset int64, whence int) (int64, error) {
	position, err := r.source.Seek(offset, whence)
	if err != nil {
		return position, err
	}
	r.current = position
	r.report()
	return position, nil
}

func (r *ProgressReadSeeker) report() {
	if r.callback == nil {
		return
	}
	r.callback(TransferProgress{Current: r.current, Total: r.total})
}
