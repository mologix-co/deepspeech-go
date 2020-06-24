package main

import (
	"fmt"
	ds "github.com/mologix-co/deepspeech-go"
	"github.com/mologix-co/deepspeech-go/example/audio"
	"github.com/pidato/vad-go"
	"io"
	"runtime"
	"time"
)

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
						stream.FeedAudioContent(p)
					})
				}
				begin = time.Now()
				stream.FeedAudioContent(buf)
				end = time.Now()
				feedDur += end.Sub(begin)

				if utteranceCount%10 == 0 {
					begin = time.Now()

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
		}

		fmt.Println()
		fmt.Printf("\tWav Duration: %v\n", reader.Elapsed())
		fmt.Printf("\tCPU Percentage: %v\n", float64(time.Now().Sub(started))/float64(reader.Elapsed()))
		fmt.Printf("\tCPU: %v\n", time.Now().Sub(started))
		fmt.Printf("\t\tFeed Duration: %v\n", feedDur)
		fmt.Printf("\t\tFinish Duration: %v\n", finishDur)
		fmt.Printf("\t\tIntermediate Duration: %v\n", intermediateDur)

		_ = reader.Close()
		_ = fileReader.Close()
		_ = v.Close()

		runtime.GC()
	}
}
