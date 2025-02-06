package request

import (
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var (
	progressManager *ProgressManager
	once            sync.Once
)

// ProgressManager manages multiple download tasks progress display
type ProgressManager struct {
	progress *mpb.Progress
	tasks    map[string]*DownloadProgress
	mutex    sync.RWMutex
}

// Get global progress manager instance
func GetProgressManager() *ProgressManager {
	once.Do(func() {
		progressManager = &ProgressManager{
			progress: mpb.New(mpb.WithWidth(60), mpb.WithRefreshRate(1*time.Second)),
			tasks:    make(map[string]*DownloadProgress),
		}
	})
	return progressManager
}

type DownloadProgress struct {
	fileName   string
	bar        *mpb.Bar
	totalSize  int64
	lastUpdate time.Time
}

func NewDownloadProgress(fileName string, totalSize, startPos int64) *DownloadProgress {
	manager := GetProgressManager()

	// Create progress bar with decorators
	bar := manager.progress.AddBar(totalSize,
		mpb.PrependDecorators(
			decor.Name(fileName, decor.WC{W: len(fileName) + 1, C: decor.DindentRight}),
			decor.CountersKiloByte("%.1f / %.1f", decor.WC{W: 20}),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 5}),
			decor.Name("] ", decor.WC{W: 1}),
			decor.EwmaSpeed(decor.SizeB1024(0), "% .2f", 30, decor.WCSyncSpace),
		),
	)

	// Set initial progress if resuming download
	if startPos > 0 {
		bar.SetCurrent(startPos)
	}

	dp := &DownloadProgress{
		fileName:   fileName,
		bar:        bar,
		totalSize:  totalSize,
		lastUpdate: time.Now(),
	}

	manager.mutex.Lock()
	manager.tasks[fileName] = dp
	manager.mutex.Unlock()

	return dp
}

func (dp *DownloadProgress) Update(n int64) {
	dp.bar.EwmaIncrBy(int(n), time.Since(dp.lastUpdate))
	dp.lastUpdate = time.Now()
}

func (dp *DownloadProgress) Success() {
	dp.bar.Completed()
	GetProgressManager().RemoveTask(dp.fileName)
}

func (dp *DownloadProgress) Fail(err error) {
	dp.bar.Abort(true)
	GetProgressManager().RemoveTask(dp.fileName)
}

// ProgressManager methods
func (pm *ProgressManager) AddTask(fileName string, task *DownloadProgress) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.tasks[fileName] = task
}

func (pm *ProgressManager) RemoveTask(fileName string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	delete(pm.tasks, fileName)
}
