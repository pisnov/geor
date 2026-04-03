Saya senang mendengarkan podcast di youtube namun karena 1 tab di chrome menghabiskan resource yang besar.
Saya berencana membuat aplikasi terminal menggunakan golang v1. 26 hanya agar bisa mendengarkan audionya saja dan jangan lupa refer ke context7. 
Saya juga ingin agar ada fitur playback speed agar saya bisa mengaturnya. 
Buatkan plan yang nantinya bisa di build oleh model yang lebih murah.9:16 PMUsed Context7 integration, loaded toolsUsed Context7 integration, loaded toolsSudah dapat semua info yang dibutuhkan. Berikut plannya:

🎧 YT-Audio Player — Terminal App (Go 1.26)
Overview Arsitektur
[User Input: URL]
       ↓
  go-ytdlp  ──── stream audio pipe (stdout) ────→  ffmpeg (decode ke PCM/MP3)
                                                          ↓
                                              gopxl/beep (playback + speed)
                                                          ↓
                                               Terminal UI (bubbletea/tview)

Stack & Dependencies
KebutuhanLibraryAlasanAmbil audio YouTubegithub.com/lrstanley/go-ytdlpGo binding resmi untuk yt-dlp, auto-install binaryPlayback audiogithub.com/gopxl/beepSupport ResampleRatio untuk speed controlTerminal UIgithub.com/charmbracelet/bubbleteaTUI modern, ringanMP3 decodegithub.com/gopxl/beep/mp3Sudah bundled dengan beep

Struktur Folder
yt-audio/
├── main.go
├── player/
│   ├── player.go       # Logic playback + speed control
│   └── stream.go       # Pipe audio dari yt-dlp ke beep
├── ui/
│   └── tui.go          # Terminal UI (input URL, controls)
├── go.mod
└── go.sum

Plan Implementasi (Per File)
main.go

Entry point
Init TUI, passing player instance


player/stream.go
Tugas: Jalankan yt-dlp via go-ytdlp, pipe output audio ke stdin ffmpeg, lalu pipe PCM ke beep.
yt-dlp -f bestaudio -o - [URL]  →  pipe  →  ffmpeg -i pipe:0 -f mp3 pipe:1  →  beep mp3.Decode(pipe)
Key points untuk builder:

Gunakan ytdlp.MustInstall(ctx, nil) untuk auto-download yt-dlp binary
Set format ke bestaudio dan output ke stdout (-o -)
Sambungkan stdout yt-dlp → stdin ffmpeg via io.Pipe
Output ffmpeg di-decode dengan beep/mp3


player/player.go
Tugas: Kontrol playback menggunakan beep.
Berdasarkan docs Context7, implementasi speed control menggunakan pattern ini:
goctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
speedy := beep.ResampleRatio(4, 1.0, ctrl)  // ratio = playback speed
speaker.Play(speedy)

// Untuk ubah speed saat runtime:
// speedy.SetRatio(1.5) // 1.5x speed
```

Methods yang perlu diekspos:
- `Play(url string)`
- `Pause() / Resume()`
- `SetSpeed(ratio float64)` — wrap `speedy.SetRatio()`
- `Stop()`
- `GetPosition() time.Duration`

---

### `ui/tui.go`
**Tugas:** TUI sederhana dengan bubbletea.

Layout terminal:
```
┌─────────────────────────────────────┐
│  🎧 YT Audio Player                 │
│  Now Playing: [judul video]         │
│  ──────────────────────────────     │
│  [████████░░░░░░░░] 2:34 / 8:12    │
│                                     │
│  Speed: 1.25x  │  Volume: 80%       │
│                                     │
│  [p] Pause  [+/-] Speed  [q] Quit  │
│  URL: _                             │
└─────────────────────────────────────┘
```

Keybinds:
| Key | Aksi |
|---|---|
| `p` | Pause/Resume |
| `+` | Speed +0.25x |
| `-` | Speed -0.25x |
| `r` | Reset speed ke 1.0x |
| `q` | Quit |
| `Enter` | Load URL baru |

---

## Prerequisites di Sistem

Builder perlu pastikan ini tersedia:
- `ffmpeg` terinstall di PATH (untuk decode audio stream)
- `yt-dlp` — **tidak perlu manual install**, `go-ytdlp` akan auto-download ke cache

---

## `go.mod` (seed)
```
module yt-audio

go 1.26

require (
    github.com/lrstanley/go-ytdlp latest
    github.com/gopxl/beep latest
    github.com/charmbracelet/bubbletea latest
)

Catatan Penting untuk Builder

Pipe audio tanpa download — kunci utamanya adalah flag -o - di yt-dlp agar audio di-stream ke stdout, bukan disimpan ke disk. Ini yang bikin resource ringan.
Speed control runtime — beep.ResampleRatio return sebuah struct dengan method SetRatio(float64). Simpan reference-nya di struct Player supaya bisa diubah kapanpun dari TUI.
Thread safety — semua operasi beep harus dibungkus speaker.Lock() / speaker.Unlock() saat mengubah state (speed, pause) dari goroutine TUI.
Error handling stream — jika URL invalid atau video private, yt-dlp akan return error di stderr. Tangkap ini dan tampilkan di TUI.