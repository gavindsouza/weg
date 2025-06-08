package tools

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

type ProgressManager struct {
	mu       sync.Mutex
	bars     map[string]*progressbar.ProgressBar
	finished map[string]bool
	barCount int
	disabled bool
}

func NewProgressManager() *ProgressManager {
	// Check if progress bars should be disabled
	disabled := os.Getenv("WEG_NO_PROGRESS") != ""

	// Clear screen and move cursor to top only if progress bars are enabled
	if !disabled {
		fmt.Fprint(os.Stderr, "\033[2J\033[H")
	}

	return &ProgressManager{
		bars:     make(map[string]*progressbar.ProgressBar),
		finished: make(map[string]bool),
		barCount: 0,
		disabled: disabled,
	}
}

func (pm *ProgressManager) AddBar(name string, total int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.disabled {
		return
	}

	if _, exists := pm.bars[name]; exists {
		return
	}

	// Create a custom writer that adds line position control
	lineWriter := &linePositionWriter{
		line: pm.barCount + 1,
		w:    os.Stderr,
	}

	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription(name),
		progressbar.OptionSetWriter(lineWriter),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetVisibility(true),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	pm.bars[name] = bar
	pm.barCount++
}

func (pm *ProgressManager) Increment(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.disabled {
		return
	}

	if bar, exists := pm.bars[name]; exists {
		bar.Add(1)
	}
}

func (pm *ProgressManager) Finish(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.disabled {
		return
	}

	if bar, exists := pm.bars[name]; exists {
		bar.Finish()
		pm.finished[name] = true
	}
}

func (pm *ProgressManager) IsFinished(name string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.finished[name]
}

func (pm *ProgressManager) WaitForAll() {
	if pm.disabled {
		return
	}

	for {
		allFinished := true
		for name := range pm.bars {
			if !pm.IsFinished(name) {
				allFinished = false
				break
			}
		}
		if allFinished {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// linePositionWriter is a custom writer that ensures output stays on a specific line
type linePositionWriter struct {
	line int
	w    *os.File
}

func (w *linePositionWriter) Write(p []byte) (n int, err error) {
	// Move to the correct line before writing
	fmt.Fprintf(w.w, "\033[%d;0H", w.line)
	return w.w.Write(p)
}
