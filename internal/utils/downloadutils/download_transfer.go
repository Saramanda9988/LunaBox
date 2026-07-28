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
	"lunabox/internal/version"

	grab "github.com/cavaliergopher/grab/v3"
	"github.com/zeebo/blake3"
)

const (
	multipartDownloadMinSize    int64 = 32 * 1024 * 1024
	multipartDownloadMinPart    int64 = 8 * 1024 * 1024
	multipartDownloadMaxParts         = 8
	multipartStateVersion             = 1
	transientReadRetryDelay           = 1200 * time.Millisecond
	maxTransientReadRetries           = 3
	defaultRetryAfter                 = 2 * time.Second
	minRetryAfter                     = 100 * time.Millisecond
	grabMax429Retries                 = 5
	multipartMax429RetriesAtOne       = 5
)

var errMultipartUnsupported = errors.New("multipart download unsupported")

type rateLimitError struct {
	retryAfter time.Duration
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("server returned %d %s", http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
}

type Progress struct {
	Downloaded int64
	Total      int64
}

type TransferConfig struct {
	ProxyConfig proxyutils.ProxyConfigProvider
	UserAgent   string // 可选的完整自定义 User-Agent
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
		userAgent = version.UserAgent()
	}

	proxyMode := proxyutils.ProxyModeSystem
	proxyURL := ""
	if config.ProxyConfig != nil {
		proxyMode, proxyURL = config.ProxyConfig.NetworkProxyConfig()
	}

	httpClient, proxyDesc, err := newSecureHTTPClient(proxyMode, proxyURL)
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

func NewSecureHTTPClientFromConfig(timeout time.Duration, config proxyutils.ProxyConfigProvider) (*http.Client, string, error) {
	proxyMode := proxyutils.ProxyModeSystem
	proxyURL := ""
	if config != nil {
		proxyMode, proxyURL = config.NetworkProxyConfig()
	}

	httpClient, proxyDesc, err := newSecureHTTPClient(proxyMode, proxyURL)
	if err != nil {
		return nil, "", err
	}
	httpClient.Timeout = timeout
	return httpClient, proxyDesc, nil
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
	if err := ensureTempDownloadPlaceholder(req.DestinationPath); err != nil {
		return fmt.Errorf("create download placeholder: %w", err)
	}

	// 下载统一先写入 .lunabox.download 临时文件，校验通过后原子重命名到最终路径。
	// 这样最终路径上只要文件存在就一定是完整且校验通过的，续传/校验逻辑也
	// 永远不会把用户已有的同名文件当成部分下载去追加或删除。
	session, ok := d.prepareMultipartSession(ctx, req)
	if ok {
		err := d.downloadWithMultipart(ctx, req, session)
		if err == nil {
			return finalizeDownloadedFile(req.DestinationPath)
		}
		if !errors.Is(err, errMultipartUnsupported) {
			return err
		}
	}

	if err := d.downloadWithGrab(ctx, req); err != nil {
		return err
	}
	return finalizeDownloadedFile(req.DestinationPath)
}

// ensureTempDownloadPlaceholder 让单流和分片下载都在最终文件旁有一个可见、
// 路径稳定的占位文件。只使用 O_CREATE、不使用 O_TRUNC，避免截断已有断点数据。
func ensureTempDownloadPlaceholder(destPath string) error {
	file, err := os.OpenFile(TempDownloadPath(destPath), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}

// finalizeDownloadedFile 把校验通过的临时下载文件原子重命名到最终路径。
// 最终路径已存在时会被覆盖（同名重新下载的场景）。
func finalizeDownloadedFile(destPath string) error {
	if err := os.Rename(TempDownloadPath(destPath), destPath); err != nil {
		return fmt.Errorf("finalize downloaded file: %w", err)
	}
	return nil
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

	// 临时文件里已有单流部分数据（例如之前多线程降级后的续传数据），
	// 交给 grab 续传，不再开新的多线程会话
	if fileInfo, statErr := os.Stat(TempDownloadPath(req.DestinationPath)); statErr == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
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

	concurrency := len(session.parts)
	if concurrency < 1 {
		concurrency = 1
	}
	rateLimitedAtOne := 0
	transientReadRetries := 0
	lastTransientProgress := int64(-1)
	for {
		pending, err := session.pendingSegments()
		if err != nil {
			return fmt.Errorf("inspect multipart state: %w", err)
		}
		if len(pending) == 0 {
			break
		}

		if concurrency > len(pending) {
			concurrency = len(pending)
		}

		err = d.downloadMultipartWave(ctx, req.URL, session, pending, concurrency, &downloaded, req.Progress)
		if err == nil {
			rateLimitedAtOne = 0
			transientReadRetries = 0
			continue
		}
		if errors.Is(err, errMultipartUnsupported) {
			// 降级到单线程下载前，把已完成的连续前缀合并进临时文件，
			// 让 grab 从该偏移续传，而不是丢弃全部分段进度。
			// 合并中途出错也无妨：已写入的字节仍是合法的连续前缀。
			_ = session.salvageContiguousPrefix()
			_ = os.RemoveAll(MultipartTempDir(session.destPath))
			return err
		}

		var limitErr *rateLimitError
		if errors.As(err, &limitErr) {
			if waitErr := waitForRetryAfter(ctx, limitErr.retryAfter); waitErr != nil {
				return waitErr
			}
			nextConcurrency := reduceMultipartConcurrency(concurrency)
			if nextConcurrency == 1 && concurrency == 1 {
				rateLimitedAtOne++
				if rateLimitedAtOne > multipartMax429RetriesAtOne {
					return err
				}
			} else {
				rateLimitedAtOne = 0
			}
			concurrency = nextConcurrency
			continue
		}
		if isRetryableDownloadReadError(err) {
			// 有新进展就重置计数，避免长下载过程中累计瞬时错误导致失败
			if downloaded.Load() > lastTransientProgress {
				transientReadRetries = 0
				lastTransientProgress = downloaded.Load()
			}
			transientReadRetries++
			if transientReadRetries <= maxTransientReadRetries {
				if waitErr := waitForRetryAfter(ctx, transientReadRetryDelay); waitErr != nil {
					return waitErr
				}
				continue
			}
		}

		return err
	}

	tempPath := TempDownloadPath(session.destPath)
	if err := session.mergeIntoTempFile(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("merge multipart files: %w", err)
	}
	if err := verifyDownloadedFileChecksum(tempPath, req.ChecksumAlgo, req.Checksum); err != nil {
		_ = os.Remove(tempPath)
		_ = os.RemoveAll(MultipartTempDir(session.destPath))
		return fmt.Errorf("checksum verify failed: %w", err)
	}
	_ = os.RemoveAll(session.tempDir)

	emitProgress(req.Progress, session.size, session.size)
	return nil
}

func (d *Downloader) downloadMultipartWave(
	ctx context.Context,
	rawURL string,
	session *multipartSession,
	segments []multipartSegment,
	concurrency int,
	downloaded *atomic.Int64,
	progress func(Progress),
) error {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(segments) {
		concurrency = len(segments)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan multipartSegment, len(segments))
	for _, segment := range segments {
		jobs <- segment
	}
	close(jobs)

	errCh := make(chan error, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for segment := range jobs {
				if workerCtx.Err() != nil {
					return
				}
				if err := d.downloadMultipartSegment(workerCtx, rawURL, segment, session.partPath(segment.Index), downloaded); err != nil {
					if errors.Is(err, context.Canceled) {
						if ctx.Err() != nil {
							select {
							case errCh <- ctx.Err():
							default:
							}
						}
						return
					}
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
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
			emitProgress(progress, downloaded.Load(), session.size)
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

	emitProgress(progress, downloaded.Load(), session.size)
	// 优先返回外层 ctx 的取消原因：暂停/取消触发的连接中断在 body 读取层
	// 可能表现为 "use of closed network connection" 等错误，不能让它们
	// 掩盖真实的 context.Canceled，否则上层无法识别为用户主动中断
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if firstErr != nil {
		return firstErr
	}
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
	if resp.StatusCode == http.StatusTooManyRequests {
		return newRateLimitError(resp)
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
	tempPath := TempDownloadPath(req.DestinationPath)
	rangeResetRetried := false
	rateLimitRetries := 0
	transientReadRetries := 0
	lastTransientProgress := int64(-1)
	for {
		resp, err := d.runGrabAttempt(ctx, req)
		if err != nil && !rangeResetRetried && shouldRetryGrabFromScratch(err, tempPath) {
			rangeResetRetried = true
			if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("reset partial download: %w", removeErr)
			}
			_ = os.RemoveAll(MultipartTempDir(req.DestinationPath))
			emitProgress(req.Progress, 0, req.ExpectedSize)
			continue
		}
		if resp != nil {
			emitProgress(req.Progress, resp.BytesComplete(), totalFromGrabResponse(resp, req.ExpectedSize))
		}
		if err != nil && isGrabRateLimited(resp, err) {
			rateLimitRetries++
			if rateLimitRetries > grabMax429Retries {
				return err
			}
			if waitErr := waitForRetryAfter(ctx, retryAfterFromResponse(resp.HTTPResponse)); waitErr != nil {
				return waitErr
			}
			continue
		}
		if err != nil && isRetryableDownloadReadError(err) {
			// 只要相比上次瞬时错误有新的下载进展，就重置重试计数：
			// 断续网络下大文件不会因为累计几次 EOF 就整体失败
			if resp != nil && resp.BytesComplete() > lastTransientProgress {
				transientReadRetries = 0
				lastTransientProgress = resp.BytesComplete()
			}
			transientReadRetries++
			if transientReadRetries <= maxTransientReadRetries {
				if waitErr := waitForRetryAfter(ctx, transientReadRetryDelay); waitErr != nil {
					return waitErr
				}
				continue
			}
		}
		return err
	}
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
	// grab 始终写临时文件，校验通过后由 Download 统一重命名到最终路径
	grabReq, err := grab.NewRequest(TempDownloadPath(req.DestinationPath), req.URL)
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
	// deleteOnError=true：校验失败说明本地文件已损坏，必须删除，
	// 否则重试时 grab 会看到"大小一致"的损坏文件而永远校验失败
	grabReq.SetChecksum(checksumHash, checksumBytes, true)

	return grabReq, nil
}

func MultipartTempDir(destPath string) string {
	return destPath + ".lunabox.parts"
}

// TempDownloadPath 下载过程中的临时文件路径（类似浏览器的 .crdownload/.part）。
// 命名是确定性的，应用重启后依然能定位到同一个文件继续断点续传。
func TempDownloadPath(destPath string) string {
	return destPath + ".lunabox.download"
}

func InspectResumeOffset(destPath string, expectedSize int64) int64 {
	if session, exists, err := loadMultipartSession(destPath, expectedSize); err == nil && exists {
		if bytes, completedErr := session.completedBytes(); completedErr == nil {
			return bytes
		}
	}

	// 单流部分数据存放在临时下载文件里，最终路径不参与续传
	tempPath := TempDownloadPath(destPath)
	fileInfo, err := os.Stat(tempPath)
	if err != nil || fileInfo.IsDir() {
		return 0
	}
	if expectedSize > 0 && fileInfo.Size() > expectedSize {
		_ = os.Remove(tempPath)
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

func (s *multipartSession) pendingSegments() ([]multipartSegment, error) {
	pending := make([]multipartSegment, 0, len(s.parts))
	for _, segment := range s.parts {
		partLength := segment.End - segment.Start + 1
		size, err := currentPartSize(s.partPath(segment.Index), partLength)
		if err != nil {
			return nil, err
		}
		if size < partLength {
			pending = append(pending, segment)
		}
	}
	return pending, nil
}

// mergeIntoTempFile 把全部分段按顺序合并进临时下载文件（校验通过后再由上层重命名到最终路径）
func (s *multipartSession) mergeIntoTempFile() error {
	file, err := os.OpenFile(TempDownloadPath(s.destPath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open temp download file: %w", err)
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
		return fmt.Errorf("stat temp download file: %w", err)
	}
	if info.Size() != s.size {
		return fmt.Errorf("merged file size mismatch: expected=%d got=%d", s.size, info.Size())
	}

	return nil
}

// salvageContiguousPrefix 把已下载的连续前缀（完整的前导分段 + 第一个不完整
// 分段的已有字节）合并进临时下载文件，供多线程降级到单线程后继续断点续传。
// 第一个空洞之后的分段数据无法作为前缀使用，会被放弃。
func (s *multipartSession) salvageContiguousPrefix() error {
	file, err := os.OpenFile(TempDownloadPath(s.destPath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open temp download file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 256*1024)
	for _, segment := range s.parts {
		partLength := segment.End - segment.Start + 1
		size, err := currentPartSize(s.partPath(segment.Index), partLength)
		if err != nil {
			return err
		}
		if size == 0 {
			break
		}

		partFile, err := os.Open(s.partPath(segment.Index))
		if err != nil {
			return fmt.Errorf("open multipart segment: %w", err)
		}
		_, copyErr := io.CopyBuffer(file, partFile, buffer)
		partFile.Close()
		if copyErr != nil {
			return fmt.Errorf("merge multipart segment: %w", copyErr)
		}
		if size < partLength {
			break
		}
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

	// 已有部分文件时的续传失败都视为"本地部分文件已不可续传"，删掉重新下载：
	// - 416：本地文件比服务端大或服务器不接受该 Range
	// - ErrBadLength：服务器对 Range 请求返回了 200 全量（或大小与预期不符），
	//   继续 append 只会得到损坏文件
	if errors.Is(err, grab.ErrBadLength) {
		return true
	}

	var statusErr grab.StatusCodeError
	return errors.As(err, &statusErr) && int(statusErr) == http.StatusRequestedRangeNotSatisfiable
}

func isRetryableDownloadReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return message == "eof" || strings.Contains(message, "unexpected eof")
}

func isGrabRateLimited(resp *grab.Response, err error) bool {
	if resp == nil || resp.HTTPResponse == nil {
		return false
	}
	if resp.HTTPResponse.StatusCode != http.StatusTooManyRequests {
		return false
	}
	var statusErr grab.StatusCodeError
	return errors.As(err, &statusErr) && int(statusErr) == http.StatusTooManyRequests
}

func newRateLimitError(resp *http.Response) error {
	return &rateLimitError{retryAfter: retryAfterFromResponse(resp)}
}

func retryAfterFromResponse(resp *http.Response) time.Duration {
	if resp == nil {
		return defaultRetryAfter
	}
	return parseRetryAfter(resp.Header.Get("Retry-After"))
}

func parseRetryAfter(value string) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultRetryAfter
	}

	if seconds, err := time.ParseDuration(trimmed + "s"); err == nil {
		if seconds >= 0 {
			return seconds
		}
	}

	if when, err := http.ParseTime(trimmed); err == nil {
		wait := time.Until(when)
		if wait >= 0 {
			return wait
		}
		return 0
	}

	return defaultRetryAfter
}

func waitForRetryAfter(ctx context.Context, delay time.Duration) error {
	if delay < 0 {
		delay = defaultRetryAfter
	}
	if delay == 0 {
		delay = minRetryAfter
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func reduceMultipartConcurrency(current int) int {
	if current <= 1 {
		return 1
	}

	next := current / 2
	if next < 1 {
		next = 1
	}
	if next == current {
		next = current - 1
	}
	if next < 1 {
		next = 1
	}
	return next
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
	selection, proxyDesc, err := proxyutils.ResolveProxy(proxyMode, proxyURL)
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
