package main

import (
	"context"
	"fmt"
	ds "github.com/mologix-co/deepspeech-go"
	"github.com/mologix-co/deepspeech-go/example/audio"
	deepspeech "github.com/mologix-co/deepspeech-go/model"
	"github.com/pidato/vad-go"
	"io"
	"runtime"
	"sync"
	"time"
)

func main1() {
	m, err := ds.Open(
		"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.pbmm",
		"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.scorer",
		ds.DefaultConfig(),
	)
	if err != nil {
		panic(err)
	}
	defer m.Close()

	fmt.Println(runtime.NumCPU() / 2 + (runtime.NumCPU() / 4))

	iterations := 100
	goroutines :=  runtime.NumCPU() / 2 + (runtime.NumCPU() / 4)
	sound, err := toPCM("recording.wav")
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		start(i, m, iterations, sound, &wg)
	}

	wg.Wait()
}

func main() {
	m, err := ds.Open(
		"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.pbmm",
		"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.scorer",
		ds.DefaultConfig(),
	)
	if err != nil {
		panic(err)
	}
	defer m.Close()

	fmt.Println(ds.Version())

	file := "recording.wav"

	fmt.Println()
	fmt.Printf("DeepSpeech Model SampleRate: %d\n", m.SampleRate())

	for b := 0; b < 100; b++ {
		fmt.Println()
		fmt.Println()
		fmt.Println("Starting...")
		fileReader, err := audio.OpenWavFile(file, audio.Ptime20)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Wav Sample Rate: %d\n", fileReader.SampleRate())

		var stream ds.Stream

		reader := audio.NewReplayReader(fileReader, time.Millisecond*80)

		v := vad.New()
		v.SetSampleRate(int32(m.SampleRate()))
		v.SetMode(vad.Aggressive)

		count := 0
		utteranceCount := 0
		speaking := false
		started := time.Now()

		feedDur := time.Duration(0)
		intermediateDur := time.Duration(0)
		finishDur := time.Duration(0)

		lastIntermediate := ""

		_ = intermediateDur
		_ = lastIntermediate

		var begin time.Time
		var end time.Time

		var vadState vad.Result
		frameCount := 0
		feedCount := 0
		finishCount := 0
		intermediateCount := 0
		for {
			buf, err := reader.ReadFrame()
			if err != nil && err != io.EOF {
				panic(err)
			}
			if len(buf) == 0 {
				break
			}
			if err == io.EOF {
				break
			}
			if len(buf) != reader.FrameSize() {
				break
			}

			count++
			vadState = v.Process(buf)

			switch vadState {
			case vad.Active:
				if stream == nil {
					stream, err = m.CreateStream()
				}
				if !speaking {
					silentFrames := utteranceCount
					utteranceCount = 0
					speaking = true
					fmt.Println()
					fmt.Printf("\tSilent For: %d\n", time.Duration(silentFrames)*reader.Ptime())
					fmt.Println("\tSpeaking...")

					// Replay at most the number of silent frames so we don't overlap speech.
					_ = silentFrames
					utteranceCount = reader.Replay(silentFrames, func(p []int16) {
						//stream.FeedAudioContent(p)
						feedCount++
					})
				}

				feedCount++
				begin = time.Now()
				stream.FeedAudioContent(buf)
				end = time.Now()
				feedDur += end.Sub(begin)

				if utteranceCount%10 == 0 {
					begin = time.Now()

					intermediateCount++
					//intermediate2 := stream.IntermediateDecodeWithMetadata(10)
					intermediate := stream.IntermediateDecode()
					end = time.Now()
					intermediateDur += end.Sub(begin)
					if len(intermediate) > 0 && lastIntermediate != intermediate {
						fmt.Printf("\t\t\t%s\n", intermediate)
					}
					lastIntermediate = intermediate
				}
				utteranceCount++

			case vad.NonActive:
				if speaking {
					speechFrames := utteranceCount
					utteranceCount = 0

					start := time.Now()
					//fmt.Println("\tProcessing utterance:")
					//fmt.Printf("\t\tFrames: %d\n", speechFrames)
					//fmt.Printf("\t\tDuration: %v\n", time.Duration(speechFrames)*audio.Ptime10ms)
					speaking = false
					if stream != nil {
						finishCount++
						begin = time.Now()
						hyp := stream.FinishStreamWithBestHypothesis(5)
						stream = nil
						fmt.Printf("\t\tText: %v\n", hyp.Text)
						fmt.Printf("\t\tDur:  %v\n", hyp.Duration)
						end = time.Now()
						finishDur += end.Sub(begin)
					}
					_ = time.Now().Sub(start)
					_ = speechFrames
					//fmt.Printf("\tTime %v\n", time.Now().Sub(start))
					utteranceCount = 0
				}
				utteranceCount++

			default:
				// Invalid Frame length.
				//panic(errors.New("unknown VAD state"))
			}

			frameCount++
		}

		fmt.Println()
		fmt.Printf("\tWav Duration: %v\n", reader.Elapsed())
		fmt.Printf("\tCPU Percentage: %v\n", float64(time.Now().Sub(started))/float64(reader.Elapsed()))
		fmt.Printf("\tCPU: %v\n", time.Now().Sub(started))
		fmt.Printf("\t\tFrame Duration: 		%v\n", reader.Ptime())
		//fmt.Printf("\t\tFeed Duration: %v\n", feedDur)
		//fmt.Printf("\t\tFeed Count: %v\n", feedCount)
		fmt.Printf("\t\tFrame Feed Duration: 		%v\n", feedDur / time.Duration(feedCount))
		//fmt.Printf("\t\tFinish Duration: %v\n", finishDur)
		//fmt.Printf("\t\tFinish Count: %v\n", finishCount)
		fmt.Printf("\t\tFinish Duration: 		%v\n", finishDur / time.Duration(finishCount))
		fmt.Printf("\t\tIntermediate Duration: 		%v\n", intermediateDur / time.Duration(intermediateCount))
		//fmt.Printf("\t\tIntermediate Duration: %v\n", intermediateDur)

		_ = reader.Close()
		_ = fileReader.Close()
		_ = v.Close()

		runtime.GC()
	}
}

func toPCM(wavFilePath string) (*soundData, error) {
	file, err := audio.OpenWavFile(wavFilePath, audio.Ptime20)
	if err != nil {
		return nil, err
	}

	var data [][]int16

	for {
		frame, err := file.ReadFrame()
		if err != nil {
			if len(frame) > 0 {
				data = append(data, frame)
			}
			break
		}
		data = append(data, frame)

		for len(frame) < file.FrameSize() {
			frame = append(frame, 0)
		}
	}

	return &soundData{
		frames:     data,
		sampleRate: file.SampleRate(),
		ptime:      file.Ptime(),
		duration:   file.Elapsed(), //time.Duration(len(data)) * time.Millisecond * file.Ptime(),
	}, nil
}

type soundData struct {
	frames     [][]int16
	sampleRate int
	ptime      time.Duration
	duration   time.Duration
}

type frameReader struct {
	frames [][]int16
	index  int
}

func newFrameReader(frames [][]int16) *frameReader {
	f := &frameReader{
		frames: frames,
		index:  0,
	}
	return f
}

func (f *frameReader) Read() []int16 {
	if f.index >= len(f.frames) {
		return nil
	}
	r := f.frames[f.index]
	f.index++
	return r
}

func (f *frameReader) Replay(limit int, fn func(frame []int16)) int {
	if fn == nil {
		return 0
	}
	if limit < 0 {
		return 0
	}
	if limit == 0 {
		limit = len(f.frames)
	} else if limit > len(f.frames) {
		limit = len(f.frames)
	}

	end := f.index - 1
	start := end - limit

	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	n := 0
	for i := start; i < end; i++ {
		idx := i % len(f.frames)
		buf := f.frames[idx]
		if len(buf) > 0 {
			fn(buf)
		}
		n++
	}
	return n
}

type msg struct {
	id  int
	cpu float64
}

type hypervisor struct {
	cores []*core
	ch    chan msg

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	targetCoreLoad float64 // CPU percentage
	avgCoreLoad    float64
	minCoreLoad    float64
	maxCoreLoad    float64
}

func startHypervisor() *hypervisor {
	ctx, cancel := context.WithCancel(context.Background())
	h := &hypervisor{
		cores:  make([]*core, 0, runtime.NumCPU()*4),
		ctx:    ctx,
		cancel: cancel,
	}
	h.wg.Add(1)
	go h.run()
	return h
}

func (h *hypervisor) run() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case m := <-h.ch:
			_ = m

		case <-time.After(time.Millisecond * 100):

		}
	}
}

type core struct {
	kernel *hypervisor
	id     int
	model  deepspeech.Model
	sound  *soundData

	ptime int

	iterations          int
	count               int64
	elapsed             int64
	cpuPercentage       float64
	streamsPerCPU       float64
	feedElapsed         int64
	finishElapsed       int64
	intermediateElapsed int64

	avgLoad float64

	wg      *sync.WaitGroup
	wgLocal sync.WaitGroup

	mu sync.Mutex
}

func start(id int, model deepspeech.Model, iterations int, sound *soundData, wg *sync.WaitGroup) *core {
	r := &core{
		id:    id,
		model: model,
		sound: sound,

		iterations: iterations,
		wg:         wg,
	}

	r.wg.Add(1)
	r.wgLocal.Add(1)
	go r.run()
	return r
}

func (r *core) run() {
	defer func() {
		r.wgLocal.Done()
		r.wg.Done()
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	//m, _ := ds.Open(
	//	"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.pbmm",
	//	"/opt/deepspeech/0.7.4/deepspeech-0.7.4-models.scorer",
	//	ds.DefaultConfig(),
	//)
	m := r.model
	//ptime := r.sound.ptime

	iterations := int(r.iterations)
	for i := 0; i < iterations; i++ {
		v := vad.New()
		v.SetSampleRate(int32(r.sound.sampleRate))
		v.SetMode(vad.Aggressive)

		reader := newFrameReader(r.sound.frames)

		count := 0
		utteranceCount := 0
		speaking := false
		started := time.Now()

		feedDur := time.Duration(0)
		intermediateDur := time.Duration(0)
		finishDur := time.Duration(0)

		lastIntermediate := ""

		_ = intermediateDur
		_ = lastIntermediate

		var stream deepspeech.Stream
		var begin time.Time
		var end time.Time
		var vadState vad.Result
		var err error

		for {
			frame := reader.Read()
			if len(frame) == 0 {
				break
			}
			count++
			vadState = v.Process(frame)

			switch vadState {
			case vad.Active:
				if stream == nil {
					stream, err = m.CreateStream()
					if err != nil {
						panic(err)
					}
				}
				if !speaking {
					silentFrames := utteranceCount
					utteranceCount = 0
					speaking = true
					//fmt.Println()
					//fmt.Printf("\tSilent For: %d\n", time.Duration(silentFrames)*ptime)
					//fmt.Println("\tSpeaking...")

					// Replay at most the number of silent frames so we don't overlap speech.
					_ = silentFrames
					utteranceCount = reader.Replay(silentFrames, func(p []int16) {
						stream.FeedAudioContent(p)
					})
				}
				begin = time.Now()
				stream.FeedAudioContent(frame)
				end = time.Now()
				feedDur += end.Sub(begin)

				if utteranceCount%10 == 0 {
					begin = time.Now()

					//intermediate2 := stream.IntermediateDecodeWithMetadata(10)
					intermediate := stream.IntermediateDecode()
					end = time.Now()
					intermediateDur += end.Sub(begin)
					if len(intermediate) > 0 && lastIntermediate != intermediate {
						//fmt.Printf("\t\t\t%s\n", intermediate)
					}
					lastIntermediate = intermediate
				}
				utteranceCount++

			case vad.NonActive:
				if speaking {
					speechFrames := utteranceCount
					utteranceCount = 0

					start := time.Now()
					//fmt.Println("\tProcessing utterance:")
					//fmt.Printf("\t\tFrames: %d\n", speechFrames)
					//fmt.Printf("\t\tDuration: %v\n", time.Duration(speechFrames)*audio.Ptime10ms)
					speaking = false
					if stream != nil {
						begin = time.Now()
						hyp := stream.FinishStreamWithBestHypothesis(1)
						stream = nil
						//fmt.Printf("\t\tText: %v\n", hyp.Text)
						//fmt.Printf("\t\tDur:  %v\n", hyp.Duration)
						end = time.Now()
						finishDur += end.Sub(begin)
						_ = hyp
					}
					_ = time.Now().Sub(start)
					_ = speechFrames
					//fmt.Printf("\tTime %v\n", time.Now().Sub(start))
					utteranceCount = 0
				}
				utteranceCount++

			default:
				// Invalid Frame length.
				//panic(errors.New("unknown VAD state"))
			}
		}

		end = time.Now()
		cpu := 1.0 / (float64(end.Sub(started)) / float64(r.sound.duration))

		fmt.Println()
		//fmt.Printf("\tCPU Percentage: %v\n", float64(time.Now().Sub(started))/float64(reader.Elapsed()))
		fmt.Printf("\tCPU Percentage: %v\n", cpu)
		//fmt.Printf("\tFrames: %d\n", r.sound.duration)

		//r.kernel.ch <- msg{
		//	id:  r.id,
		//	cpu: cpu,
		//}

		//fmt.Printf("\tCPU: %v\n", time.Now().Sub(started))
		//fmt.Printf("\t\tFeed Duration: %v\n", feedDur)
		//fmt.Printf("\t\tFinish Duration: %v\n", finishDur)
		//fmt.Printf("\t\tIntermediate Duration: %v\n", intermediateDur)

		v.Reset()
		_ = v.Close()
	}

}
