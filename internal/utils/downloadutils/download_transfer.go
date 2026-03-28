package downloadutils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lunabox/internal/utils/proxyutils"

	grab "github.com/cavaliergopher/grab/v3"
	"github.com/zeebo/blake3"
)

const (
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 LunaBox/1.0"

	multipartDownloadMinSize  int64 = 32 * 1024 * 1024
	multipartDownloadMinPart  int64 = 8 * 1024 * 1024
	multipartDownloadMaxParts       = 8
	multipartStateVersion           = 1
)

var errMultipartUnsupported = errors.New("multipart download unsupported")

type Progress struct {
	Downloaded int64
	Total      int64
}

type TransferConfig struct {
	ProxyMode string
	ProxyURL  string
	UserAgent string
}

type TransferRequest struct {
	URL             string
	DestinationPath string
	ExpectedSize    int64
	ChecksumAlgo    string
	Checksum        string
	Progress        func(Progress)
}

type Downloader struct {
	httpClient *http.Client
	grabClient *grab.Client
	userAgent  string
}

type multipartSegment struct {
	Index int   `json:"index"`
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type multipartState struct {
	Version int                `json:"version"`
	Size    int64              `json:"size"`
	Parts   []multipartSegment `json:"parts"`
}

type multipartSession struct {
	destPath     string
	tempDir      string
	manifestPath string
	size         int64
	parts        []multipartSegment
}

func NewDownloader(config TransferConfig) (*Downloader, string, error) {
	userAgent := strings.TrimSpace(config.UserAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	httpClient, proxyDesc, err := newSecureHTTPClient(config.ProxyMode, config.ProxyURL)
	if err != nil {
		return nil, "", err
	}

	grabClient := grab.NewClient()
	grabClient.HTTPClient = httpClient
	grabClient.UserAgent = userAgent

	return &Downloader{
		httpClient: httpClient,
		grabClient: grabClient,
		userAgent:  userAgent,
	}, proxyDesc, nil
}

func (d *Downloader) Download(ctx context.Context, req TransferRequest) error {
	if d == nil {
		return fmt.Errorf("downloader is nil")
	}
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("download url is required")
	}
	if strings.TrimSpace(req.DestinationPath) == "" {
		return fmt.Errorf("download destination path is required")
	}

	session, ok := d.prepareMultipartSession(ctx, req)
	if ok {
		if err := d.downloadWithMultipart(ctx, req, session); err != nil {
			if !errors.Is(err, errMultipartUnsupported) {
				return err
			}
		} else {
			return nil
		}
	}

	return d.downloadWithGrab(ctx, req)
}

func (d *Downloader) prepareMultipartSession(ctx context.Context, req TransferRequest) (*multipartSession, bool) {
	if req.ExpectedSize < multipartDownloadMinSize {
		return nil, false
	}

	session, exists, err := loadMultipartSession(req.DestinationPath, req.ExpectedSize)
	if err != nil {
		_ = os.RemoveAll(MultipartTempDir(req.DestinationPath))
		return nil, false
	}
	if exists {
		return session, true
	}

	if fileInfo, statErr := os.Stat(req.DestinationPath); statErr == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
		return nil, false
	}

	supported, err := d.probeMultipartSupport(ctx, req)
	if err != nil || !supported {
		return nil, false
	}

	session, err = createMultipartSession(req.DestinationPath, req.ExpectedSize)
	if err != nil {
		return nil, false
	}
	return session, true
}

func (d *Downloader) downloadWithMultipart(ctx context.Context, req TransferRequest, session *multipartSession) error {
	initialDownloaded, err := session.completedBytes()
	if err != nil {
		return fmt.Errorf("inspect multipart state: %w", err)
	}

	var downloaded atomic.Int64
	downloaded.Store(initialDownloaded)
	emitProgress(req.Progress, downloaded.Load(), session.size)

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(session.parts))
	var wg sync.WaitGroup
	for _, segment := range session.parts {
		segment := segment
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.downloadMultipartSegment(workerCtx, req.URL, segment, session.partPath(segment.Index), &downloaded); err != nil {
				errCh <- err
			}
		}()
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var firstErr error
loop:
	for {
		select {
		case <-ticker.C:
			emitProgress(req.Progress, downloaded.Load(), session.size)
		case err := <-errCh:
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
			}
		case <-doneCh:
			break loop
		}
	}

drainErrors:
	for {
		select {
		case err := <-errCh:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		default:
			break drainErrors
		}
	}

	emitProgress(req.Progress, downloaded.Load(), session.size)

	if firstErr != nil {
		if errors.Is(firstErr, errMultipartUnsupported) {
			_ = os.RemoveAll(MultipartTempDir(session.destPath))
		}
		return firstErr
	}

	if err := session.mergeIntoDestination(); err != nil {
		_ = os.Remove(session.destPath)
		return fmt.Errorf("merge multipart files: %w", err)
	}
	if err := verifyDownloadedFileChecksum(session.destPath, req.ChecksumAlgo, req.Checksum); err != nil {
		_ = os.Remove(session.destPath)
		_ = os.RemoveAll(MultipartTempDir(session.destPath))
		return fmt.Errorf("checksum verify failed: %w", err)
	}
	_ = os.RemoveAll(session.tempDir)

	emitProgress(req.Progress, session.size, session.size)
	return nil
}

func (d *Downloader) downloadMultipartSegment(ctx context.Context, rawURL string, segment multipartSegment, partPath string, downloaded *atomic.Int64) error {
	partLength := segment.End - segment.Start + 1
	existing, err := currentPartSize(partPath, partLength)
	if err != nil {
		return err
	}
	if existing >= partLength {
		return nil
	}

	rangeStart := segment.Start + existing
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create multipart request: %w", err)
	}
	httpReq.Header.Set("User-Agent", d.userAgent)
	httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, segment.End))

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return errMultipartUnsupported
	}
	if resp.StatusCode != http.StatusPartialContent {
		return grab.StatusCodeError(resp.StatusCode)
	}

	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open multipart file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 256*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("write multipart file: %w", writeErr)
			}
			downloaded.Add(int64(n))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	finalSize, err := currentPartSize(partPath, partLength)
	if err != nil {
		return err
	}
	if finalSize != partLength {
		return fmt.Errorf("multipart segment incomplete: index=%d expected=%d got=%d", segment.Index, partLength, finalSize)
	}

	return nil
}

func (d *Downloader) probeMultipartSupport(ctx context.Context, req TransferRequest) (bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodHead, req.URL, nil)
	if err != nil {
		return false, fmt.Errorf("create head request: %w", err)
	}
	httpReq.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, nil
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes") {
		return false, nil
	}
	if resp.ContentLength > 0 && req.ExpectedSize > 0 && resp.ContentLength != req.ExpectedSize {
		return false, fmt.Errorf("multipart probe size mismatch: expected=%d got=%d", req.ExpectedSize, resp.ContentLength)
	}

	return computeMultipartPartCount(req.ExpectedSize) > 1, nil
}

func (d *Downloader) downloadWithGrab(ctx context.Context, req TransferRequest) error {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := d.runGrabAttempt(ctx, req)
		if err != nil && attempt == 0 && shouldRetryGrabFromScratch(err, req.DestinationPath) {
			if removeErr := os.Remove(req.DestinationPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("reset partial download: %w", removeErr)
			}
			emitProgress(req.Progress, 0, req.ExpectedSize)
			continue
		}
		if resp != nil {
			emitProgress(req.Progress, resp.BytesComplete(), totalFromGrabResponse(resp, req.ExpectedSize))
		}
		return err
	}

	return fmt.Errorf("download failed after retry")
}

func (d *Downloader) runGrabAttempt(ctx context.Context, req TransferRequest) (*grab.Response, error) {
	grabReq, err := d.newGrabDownloadRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp := d.grabClient.Do(grabReq)
	emitProgress(req.Progress, resp.BytesComplete(), totalFromGrabResponse(resp, req.ExpectedSize))

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			emitProgress(req.Progress, resp.BytesComplete(), totalFromGrabResponse(resp, req.ExpectedSize))
		case <-resp.Done:
			emitProgress(req.Progress, resp.BytesComplete(), totalFromGrabResponse(resp, req.ExpectedSize))
			return resp, resp.Err()
		}
	}
}

func (d *Downloader) newGrabDownloadRequest(ctx context.Context, req TransferRequest) (*grab.Request, error) {
	grabReq, err := grab.NewRequest(req.DestinationPath, req.URL)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	grabReq = grabReq.WithContext(ctx)
	grabReq.Size = req.ExpectedSize
	grabReq.IgnoreRemoteTime = true
	grabReq.HTTPRequest.Header.Set("User-Agent", d.userAgent)

	checksumHash, checksumBytes, err := newChecksumState(req.ChecksumAlgo, req.Checksum)
	if err != nil {
		return nil, fmt.Errorf("configure checksum: %w", err)
	}
	grabReq.SetChecksum(checksumHash, checksumBytes, false)

	return grabReq, nil
}

func MultipartTempDir(destPath string) string {
	return destPath + ".lunabox.parts"
}

func InspectResumeOffset(destPath string, expectedSize int64) int64 {
	if session, exists, err := loadMultipartSession(destPath, expectedSize); err == nil && exists {
		if bytes, completedErr := session.completedBytes(); completedErr == nil {
			return bytes
		}
	}

	fileInfo, err := os.Stat(destPath)
	if err != nil || fileInfo.IsDir() {
		return 0
	}
	if expectedSize > 0 && fileInfo.Size() > expectedSize {
		_ = os.Remove(destPath)
		return 0
	}
	return fileInfo.Size()
}

func FormatDownloadError(expectedSize int64, err error) string {
	switch {
	case errors.Is(err, grab.ErrBadLength):
		return fmt.Sprintf("size mismatch during download: expected=%d", expectedSize)
	case errors.Is(err, grab.ErrBadChecksum):
		return "checksum verify failed: checksum mismatch"
	default:
		return fmt.Sprintf("download failed: %v", err)
	}
}

func createMultipartSession(destPath string, size int64) (*multipartSession, error) {
	partCount := computeMultipartPartCount(size)
	if partCount <= 1 {
		return nil, fmt.Errorf("multipart part count too small")
	}

	session := &multipartSession{
		destPath:     destPath,
		tempDir:      MultipartTempDir(destPath),
		manifestPath: multipartManifestPath(destPath),
		size:         size,
		parts:        buildMultipartSegments(size, partCount),
	}
	if err := os.MkdirAll(session.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("create multipart temp dir: %w", err)
	}
	if err := session.save(); err != nil {
		return nil, err
	}
	return session, nil
}

func loadMultipartSession(destPath string, expectedSize int64) (*multipartSession, bool, error) {
	manifestPath := multipartManifestPath(destPath)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read multipart manifest: %w", err)
	}

	var state multipartState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, true, fmt.Errorf("parse multipart manifest: %w", err)
	}
	if state.Version != multipartStateVersion {
		return nil, true, fmt.Errorf("unsupported multipart manifest version: %d", state.Version)
	}
	if expectedSize > 0 && state.Size != expectedSize {
		return nil, true, fmt.Errorf("multipart manifest size mismatch: expected=%d got=%d", expectedSize, state.Size)
	}
	if len(state.Parts) == 0 {
		return nil, true, fmt.Errorf("multipart manifest has no parts")
	}

	session := &multipartSession{
		destPath:     destPath,
		tempDir:      MultipartTempDir(destPath),
		manifestPath: manifestPath,
		size:         state.Size,
		parts:        state.Parts,
	}
	if err := os.MkdirAll(session.tempDir, 0755); err != nil {
		return nil, true, fmt.Errorf("create multipart temp dir: %w", err)
	}
	if err := session.validate(); err != nil {
		return nil, true, err
	}

	return session, true, nil
}

func (s *multipartSession) save() error {
	state := multipartState{
		Version: multipartStateVersion,
		Size:    s.size,
		Parts:   s.parts,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal multipart manifest: %w", err)
	}
	if err := os.WriteFile(s.manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write multipart manifest: %w", err)
	}
	return nil
}

func (s *multipartSession) validate() error {
	var expectedStart int64
	for index, segment := range s.parts {
		if segment.Index != index {
			return fmt.Errorf("multipart segment index mismatch: expected=%d got=%d", index, segment.Index)
		}
		if segment.Start != expectedStart {
			return fmt.Errorf("multipart segment start mismatch: expected=%d got=%d", expectedStart, segment.Start)
		}
		if segment.End < segment.Start {
			return fmt.Errorf("multipart segment invalid range: index=%d start=%d end=%d", segment.Index, segment.Start, segment.End)
		}
		expectedStart = segment.End + 1
	}
	if expectedStart != s.size {
		return fmt.Errorf("multipart segments do not cover full size: expected=%d got=%d", s.size, expectedStart)
	}
	return nil
}

func (s *multipartSession) completedBytes() (int64, error) {
	var total int64
	for _, segment := range s.parts {
		partLength := segment.End - segment.Start + 1
		size, err := currentPartSize(s.partPath(segment.Index), partLength)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func (s *multipartSession) mergeIntoDestination() error {
	file, err := os.OpenFile(s.destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 256*1024)
	for _, segment := range s.parts {
		partFile, err := os.Open(s.partPath(segment.Index))
		if err != nil {
			return fmt.Errorf("open multipart segment: %w", err)
		}

		if _, err := io.CopyBuffer(file, partFile, buffer); err != nil {
			partFile.Close()
			return fmt.Errorf("merge multipart segment: %w", err)
		}
		partFile.Close()
	}

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat destination file: %w", err)
	}
	if info.Size() != s.size {
		return fmt.Errorf("merged file size mismatch: expected=%d got=%d", s.size, info.Size())
	}

	return nil
}

func (s *multipartSession) partPath(index int) string {
	return filepath.Join(s.tempDir, fmt.Sprintf("part-%03d.bin", index))
}

func multipartManifestPath(destPath string) string {
	return filepath.Join(MultipartTempDir(destPath), "state.json")
}

func computeMultipartPartCount(size int64) int {
	if size < multipartDownloadMinSize {
		return 1
	}
	partCount := int((size + multipartDownloadMinPart - 1) / multipartDownloadMinPart)
	if partCount > multipartDownloadMaxParts {
		partCount = multipartDownloadMaxParts
	}
	if partCount < 1 {
		partCount = 1
	}
	return partCount
}

func buildMultipartSegments(size int64, partCount int) []multipartSegment {
	segments := make([]multipartSegment, 0, partCount)
	partSize := (size + int64(partCount) - 1) / int64(partCount)
	var start int64
	for index := 0; index < partCount; index++ {
		end := start + partSize - 1
		if end >= size {
			end = size - 1
		}
		segments = append(segments, multipartSegment{
			Index: index,
			Start: start,
			End:   end,
		})
		start = end + 1
	}
	return segments
}

func currentPartSize(partPath string, maxSize int64) (int64, error) {
	info, err := os.Stat(partPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat multipart segment: %w", err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("multipart segment is directory: %s", partPath)
	}
	if info.Size() > maxSize {
		return 0, fmt.Errorf("multipart segment exceeds expected size: path=%s expected=%d got=%d", partPath, maxSize, info.Size())
	}
	return info.Size(), nil
}

func verifyDownloadedFileChecksum(path string, algo string, checksum string) error {
	checksumHash, checksumBytes, err := newChecksumState(algo, checksum)
	if err != nil {
		return err
	}
	if checksumHash == nil {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open downloaded file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(checksumHash, file); err != nil {
		return fmt.Errorf("hash downloaded file: %w", err)
	}

	if actual := checksumHash.Sum(nil); !equalBytes(actual, checksumBytes) {
		return grab.ErrBadChecksum
	}
	return nil
}

func newChecksumState(algo string, checksum string) (hash.Hash, []byte, error) {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	trimmedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if trimmedAlgo == "" {
		return nil, nil, nil
	}

	sum, err := hex.DecodeString(trimmedChecksum)
	if err != nil {
		return nil, nil, fmt.Errorf("decode checksum: %w", err)
	}

	switch trimmedAlgo {
	case "sha256":
		return sha256.New(), sum, nil
	case "blake3":
		return blake3.New(), sum, nil
	default:
		return nil, nil, fmt.Errorf("unsupported checksum algo: %s", algo)
	}
}

func totalFromGrabResponse(resp *grab.Response, fallback int64) int64 {
	if resp == nil {
		return fallback
	}
	if size := resp.Size(); size > 0 {
		return size
	}
	return fallback
}

func emitProgress(progress func(Progress), downloaded int64, total int64) {
	if progress == nil {
		return
	}
	progress(Progress{
		Downloaded: downloaded,
		Total:      total,
	})
}

func shouldRetryGrabFromScratch(err error, destPath string) bool {
	if strings.TrimSpace(destPath) == "" {
		return false
	}

	if info, statErr := os.Stat(destPath); statErr != nil || info.IsDir() || info.Size() <= 0 {
		return false
	}

	var statusErr grab.StatusCodeError
	return errors.As(err, &statusErr) && int(statusErr) == http.StatusRequestedRangeNotSatisfiable
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func resolveAllowedAddress(ctx context.Context, address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if isBlockedHostname(host) {
		return "", fmt.Errorf("blocked host: %s", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return "", fmt.Errorf("blocked ip: %s", host)
		}
		return net.JoinHostPort(ip.String(), port), nil
	}

	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			continue
		}
		return net.JoinHostPort(ip.IP.String(), port), nil
	}

	return "", fmt.Errorf("host %s resolved only to blocked addresses", host)
}

func newSecureHTTPClient(proxyMode string, proxyURL string) (*http.Client, string, error) {
	selection, proxyDesc, err := proxyutils.ResolveDownloadProxy(proxyMode, proxyURL)
	if err != nil {
		return nil, "", fmt.Errorf("resolve download proxy: %w", err)
	}

	allowedProxyTargets := map[string]struct{}{}
	if selection != nil {
		allowedProxyTargets = selection.AllowedDialTargets()
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		DisableCompression:  true,
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 16,
		Proxy: func(req *http.Request) (*url.URL, error) {
			if selection == nil {
				return nil, nil
			}
			return selection.Proxy(req)
		},
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			if _, ok := allowedProxyTargets[address]; ok {
				return dialer.DialContext(ctx, network, address)
			}

			resolvedAddress, err := resolveAllowedAddress(ctx, address)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, resolvedAddress)
		},
	}

	return &http.Client{
		Timeout:   0,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return ValidateDownloadURL(req.URL.String())
		},
	}, proxyDesc, nil
}
