package ffmpeg

// Quality representa uma configuração de qualidade de vídeo para HLS.
type Quality struct {
	Name         string
	Width        int
	Height       int
	VideoBitrate string
	MaxRate      string
	BufSize      string
	Bandwidth    int
}

// Qualities define as 4 qualidades alvo em ordem crescente.
// A ordem importa: o master.m3u8 será gerado nesta sequência.
var Qualities = []Quality{
	{
		Name:         "360p",
		Width:        640,
		Height:       360,
		VideoBitrate: "800k",
		MaxRate:      "856k",
		BufSize:      "1200k",
		Bandwidth:    800000,
	}, {
		Name:         "480p",
		Width:        854,
		Height:       480,
		VideoBitrate: "1400k",
		MaxRate:      "1498k",
		BufSize:      "2100k",
		Bandwidth:    1400000,
	}, {
		Name:         "720p",
		Width:        1280,
		Height:       720,
		VideoBitrate: "2800k",
		MaxRate:      "2996k",
		BufSize:      "4200k",
		Bandwidth:    2800000,
	}, {
		Name:         "1080p",
		Width:        1920,
		Height:       1080,
		VideoBitrate: "5000k",
		MaxRate:      "5350k",
		BufSize:      "7500k",
		Bandwidth:    5000000,
	},
}
