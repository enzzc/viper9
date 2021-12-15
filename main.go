package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Mediainfo struct {
	Filename  string
	BitRate   int
	FrameRate int
	Height    int
	Format    string
}

type ParamSet struct {
	Crf   int
	Br    int
	MinBr int
	MaxBr int
}

func main() {
	if len(os.Args) < 2 {
		panic("Missing argument: filename(s)")
	}

	rand.Seed(time.Now().UnixNano())

	workers, _ := strconv.Atoi(os.Getenv("WORKERS"))
	if workers == 0 {
		workers++
	}

	files := len(os.Args[1:])
	fmt.Println("Workers:", workers)
	fmt.Println("Files:", files)

	var wg sync.WaitGroup
	wg.Add(files)

	jobs := make(chan string, files)
	for _, filename := range os.Args[1:] {
		jobs <- filename
	}

	for i := 0; i < workers; i++ {
		id := i
		go worker(id, &wg, jobs)
	}

	wg.Wait()

}

func worker(id int, wg *sync.WaitGroup, jobs <-chan string) {
	ffmpegPath, _ := exec.LookPath("ffmpeg")
	for {
		select {
		case filename := <-jobs:
			logId := strconv.Itoa(rand.Intn(1000000000))
			mi, _ := mediainfo(filename)
			params := inferParams(mi)

			ext := path.Ext(filename)
			filenameDst := filename[0:len(filename)-len(ext)] + ".webm"

			cmd1 := exec.Command(
				ffmpegPath,
				"-i", filename,
				"-c:v", "libvpx-vp9",
				"-b:v", fmt.Sprintf("%dk", params.Br),
				"-minrate", fmt.Sprintf("%dk", params.MinBr),
				"-maxrate", fmt.Sprintf("%dk", params.MaxBr),
				"-crf", fmt.Sprintf("%d", params.Crf),
				"-quality", "best",
				"-pass", "1",
				"-passlogfile", logId,
				"-f", "webm",
				"-y", "/dev/null",
			)
			cmd2 := exec.Command(
				ffmpegPath,
				"-i", filename,
				"-c:v", "libvpx-vp9",
				"-b:v", fmt.Sprintf("%dk", params.Br),
				"-minrate", fmt.Sprintf("%dk", params.MinBr),
				"-maxrate", fmt.Sprintf("%dk", params.MaxBr),
				"-crf", fmt.Sprintf("%d", params.Crf),
				"-quality", "best",
				"-pass", "2",
				"-passlogfile", logId,
				"-f", "webm",
				"-y", filenameDst,
			)
			fmt.Println(cmd1)
			err := cmd1.Run()
			if err == nil {
				fmt.Println(cmd2)
				err := cmd2.Run()
				fmt.Println("Error", err)
			} else {
				fmt.Println("Error", err)
			}
			wg.Done()

		default:
			return
		}
	}
}

func mediainfo(filename string) (Mediainfo, error) {
	mediainfoTemplate := "--Inform=Video;%BitRate%::%Height%::%Format%::%FrameRate%"
	mediainfoPath, err := exec.LookPath("mediainfo")
	if err != nil {
		return Mediainfo{}, err
	}
	out, _ := exec.Command(mediainfoPath, filename, mediainfoTemplate).Output()
	parts := strings.Split(strings.Trim(string(out), "\n"), "::")
	br, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	fps, _ := strconv.ParseFloat(parts[3], 64)
	return Mediainfo{
		Filename:  filename,
		BitRate:   br,
		Height:    h,
		Format:    parts[2],
		FrameRate: int(fps),
	}, nil
}

func inferParams(m Mediainfo) *ParamSet {
	crf := 0   // Max quality
	br := 0    // kbps
	maxBr := 0 // kbps
	minBr := 0 // kbps

	switch {
	case m.Height <= 240:
		crf = 37
		br = 150
		minBr = 75
		maxBr = 218

	case m.Height <= 360:
		crf = 36
		br = 276
		minBr = 138
		maxBr = 400

	case m.Height <= 480:
		crf = 33
		br = 750
		minBr = 375
		maxBr = 1088

	case m.Height <= 720:
		crf = 32
		if m.FrameRate <= 30 { // 24, 25, 30 FPS
			br = 1024
			minBr = 512
			maxBr = 1485
		} else { // 50, 60 FPS
			br = 1800
			minBr = 900
			maxBr = 2610
		}

	case m.Height <= 1080:
		crf = 31
		if m.FrameRate <= 30 { // 24, 25, 30 FPS
			br = 1800
			minBr = 900
			maxBr = 2610
		} else { // 50, 60 FPS
			br = 3000
			minBr = 1500
			maxBr = 4350
		}

	case m.Height <= 1440:
		crf = 24
		if m.FrameRate <= 30 { // 24, 25, 30 FPS
			br = 6000
			minBr = 3000
			maxBr = 8700
		} else { // 50, 60 FPS
			br = 9000
			minBr = 4500
			maxBr = 13050
		}

	case m.Height <= 2160:
		crf = 15
		if m.FrameRate <= 30 { // 24, 25, 30 FPS
			br = 12000
			minBr = 6000
			maxBr = 17400
		} else { // 50, 60 FPS
			br = 18000
			minBr = 9000
			maxBr = 26100
		}
	}
	return &ParamSet{
		Crf:   crf,
		Br:    br,
		MinBr: minBr,
		MaxBr: maxBr,
	}
}
