package player

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

type Player struct {
	mu     sync.Mutex
	ctrl   *beep.Ctrl
	paused bool
	Title  string
	Length time.Duration

	ytdlpBin  string
	streamURL string
	speed     float64

	// playGen is incremented each time a new playback is requested.
	// A goroutine checks its own generation after slow ops and aborts if superseded,
	// preventing multiple simultaneous speaker.Play() calls (double audio).
	playGen uint64

	cancel        context.CancelFunc
	cleanup       func()
	startWallTime time.Time
	startOffset   time.Duration
	pausedAt      time.Duration
	currentFormat beep.Format
	speakerInited bool
}

func NewPlayer() *Player {
	return &Player{speed: 1.0}
}

func (p *Player) ensureBin(ctx context.Context) error {
	if p.ytdlpBin != "" {
		return nil
	}
	bin, err := InstallYtDlp(ctx)
	if err != nil {
		return err
	}
	p.ytdlpBin = bin
	return nil
}

func (p *Player) Play(url string) error {
	ctx := context.Background()

	if err := p.ensureBin(ctx); err != nil {
		log.Printf("yt-dlp install error: %v", err)
		return err
	}

	type ytMeta struct {
		Title    string  `json:"title"`
		Duration float64 `json:"duration"`
	}
	cmd := exec.CommandContext(ctx, p.ytdlpBin, "--no-warnings", "-j", url)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err == nil {
		var meta ytMeta
		if json.Unmarshal(out, &meta) == nil {
			p.Title = meta.Title
			p.Length = time.Duration(meta.Duration) * time.Second
		} else {
			p.Title = "(unknown)"
			p.Length = 0
		}
	} else {
		p.Title = "(unknown)"
		p.Length = 0
	}

	streamURL, err := GetStreamURL(ctx, p.ytdlpBin, url)
	if err != nil {
		log.Printf("GetStreamURL error: %v", err)
		return err
	}

	p.mu.Lock()
	p.streamURL = streamURL
	p.speed = 1.0
	p.paused = false
	p.mu.Unlock()

	return p.startPlayback(streamURL, 1.0, 0)
}

// startPlayback stops any existing playback and starts a new ffmpeg stream.
// A generation counter ensures that if this function is called again before
// the slow network/decode steps finish, the superseded goroutine aborts
// before calling speaker.Play() — preventing double audio.
func (p *Player) startPlayback(streamURL string, speed float64, startPos time.Duration) error {
	// Step 1: claim this generation and cancel the previous stream.
	p.mu.Lock()
	p.playGen++
	myGen := p.playGen
	if p.cancel != nil {
		p.cancel()
	}
	oldCleanup := p.cleanup
	p.cleanup = nil
	p.mu.Unlock()

	if oldCleanup != nil {
		oldCleanup()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Step 2: slow ops — network connection + first MP3 frame decode.
	stream, err := StreamAudio(ctx, streamURL, speed, startPos)
	if err != nil {
		cancel()
		log.Printf("StreamAudio error: %v", err)
		return err
	}

	beepStream, format, err := DecodeMP3(stream)
	if err != nil {
		stream.Close()
		cancel()
		log.Printf("DecodeMP3 error: %v", err)
		return err
	}

	// Step 3: generation check — abort if a newer startPlayback already started.
	p.mu.Lock()
	superseded := p.playGen != myGen
	p.mu.Unlock()

	if superseded {
		beepStream.Close()
		stream.Close()
		cancel()
		return nil
	}

	// Step 4: init speaker hardware if format changed.
	p.mu.Lock()
	needInit := !p.speakerInited || p.currentFormat != format
	p.mu.Unlock()

	if needInit {
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			beepStream.Close()
			stream.Close()
			cancel()
			log.Printf("speaker.Init error: %v", err)
			return err
		}
		p.mu.Lock()
		p.currentFormat = format
		p.speakerInited = true
		p.mu.Unlock()
	}

	var cleanOnce sync.Once
	cleanup := func() {
		cleanOnce.Do(func() {
			beepStream.Close()
			stream.Close()
		})
	}

	p.mu.Lock()
	p.ctrl = &beep.Ctrl{Streamer: beepStream, Paused: p.paused}
	p.cancel = cancel
	p.cleanup = cleanup
	p.startWallTime = time.Now()
	p.startOffset = startPos
	ctrl := p.ctrl
	p.mu.Unlock()

	// Step 5: clear old speaker buffers then start new stream atomically.
	speaker.Clear()
	speaker.Play(beep.Seq(ctrl, beep.Callback(cleanup)))
	return nil
}

func (p *Player) Pause() {
	speaker.Lock()
	if p.ctrl != nil {
		p.ctrl.Paused = true
	}
	speaker.Unlock()

	p.mu.Lock()
	p.paused = true
	p.pausedAt = p.positionLocked()
	p.mu.Unlock()
}

func (p *Player) Resume() {
	p.mu.Lock()
	p.paused = false
	p.startOffset = p.pausedAt
	p.startWallTime = time.Now()
	p.mu.Unlock()

	speaker.Lock()
	if p.ctrl != nil {
		p.ctrl.Paused = false
	}
	speaker.Unlock()
}

func (p *Player) SetSpeed(ratio float64) {
	p.mu.Lock()
	streamURL := p.streamURL
	currentPos := p.positionLocked()
	p.speed = ratio
	p.mu.Unlock()

	if streamURL == "" {
		return
	}
	go func() {
		if err := p.startPlayback(streamURL, ratio, currentPos); err != nil {
			log.Printf("SetSpeed restart error: %v", err)
		}
	}()
}

// positionLocked estimates playback position. Caller must hold p.mu.
func (p *Player) positionLocked() time.Duration {
	if p.startWallTime.IsZero() {
		return 0
	}
	if p.paused {
		return p.pausedAt
	}
	elapsed := time.Since(p.startWallTime)
	pos := p.startOffset + time.Duration(float64(elapsed)*p.speed)
	if p.Length > 0 && pos > p.Length {
		return p.Length
	}
	return pos
}

func (p *Player) GetPosition() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.positionLocked()
}

func (p *Player) Stop() {
	speaker.Lock()
	if p.ctrl != nil {
		p.ctrl.Paused = true
	}
	speaker.Unlock()
}
