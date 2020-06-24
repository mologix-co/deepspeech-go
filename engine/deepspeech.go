package main

/*
#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include "deepspeech.h"
*/
import "C"
import (
	deepspeech "github.com/mologix-co/deepspeech-go/model"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	version = ""
)

func init() {
	cstr := C.DS_Version()
	defer C.DS_FreeString(cstr)
	version = C.GoString(cstr)
}

func Version() string {
	return version
}

type tokenMetadata struct {
	text       *C.char
	timestep   C.int
	start_time C.float
}
type candidateTranscript struct {
	tokens     unsafe.Pointer
	num_tokens uint32
	confidence C.double
}

type metadata struct {
	transcripts     unsafe.Pointer
	num_transcripts uint32
}

type model struct {
	beamWidth  uint32
	sampleRate int
	state      *C.ModelState

	counter uint64
	streams map[uint64]*stream
	mu      sync.RWMutex
}

type stream struct {
	model *model
	id    uint64
	state *C.StreamingState
	mu    sync.Mutex
}

func New(modelPath string, beamWidth uint32) (deepspeech.Model, error) {
	var state *C.ModelState
	cstrModelPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cstrModelPath))
	code := int(C.DS_CreateModel(cstrModelPath, &state))
	if code != 0x0000 {
		return nil, deepspeech.ErrorOf(code)
	}

	code = int(C.DS_SetModelBeamWidth(state, C.uint32_t(beamWidth)))
	if code != 0x0000 {
		C.DS_FreeModel(state)
		return nil, deepspeech.ErrorOf(code)
	}

	return &model{
		beamWidth:  beamWidth,
		sampleRate: int(C.DS_GetModelSampleRate(state)),
		streams:    make(map[uint64]*stream, 1000),
		state:      state,
	}, nil
}

func (m *model) EnableExternalScorer(path string, aAlpha, aBeta float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cstrPath := C.CString(path)
	defer C.free(unsafe.Pointer(cstrPath))

	err := deepspeech.ErrorOf(int(C.DS_EnableExternalScorer(m.state, cstrPath)))
	if err != nil {
		return err
	}

	err = deepspeech.ErrorOf(int(C.DS_SetScorerAlphaBeta(m.state, C.float(aAlpha), C.float(aBeta))))
	return err
}

func (m *model) SampleRate() int {
	return m.sampleRate
}

func (m *model) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return os.ErrClosed
	}
	if len(m.streams) > 0 {
		return deepspeech.ErrOpenStreams
	}
	C.DS_FreeModel(m.state)
	m.state = nil
	return nil
}

func (m *model) SpeechToText(frame []int16) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cstr := C.DS_SpeechToText(
		m.state,
		(*C.short)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&frame)).Data)),
		C.uint(len(frame)))
	// Convert C string to Go string
	res := C.GoString(cstr)
	// Free C string
	C.DS_FreeString(cstr)
	return res
}

func (m *model) CreateStream() (deepspeech.Stream, error) {
	m.mu.RLock()
	if m.state == nil {
		m.mu.RUnlock()
		return nil, os.ErrClosed
	}
	var state *C.StreamingState
	code := int(C.DS_CreateStream(m.state, &state))
	if code != 0x0000 {
		m.mu.RUnlock()
		return nil, deepspeech.ErrorOf(code)
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		C.DS_FreeStream(state)
		return nil, os.ErrClosed
	}
	m.counter++
	stream := &stream{
		id:    m.counter,
		state: state,
		model: m,
	}
	m.streams[stream.id] = stream

	return stream, nil
}

func (m *model) removeStream(s *stream) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, s.id)
}

func (s *stream) Free() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return os.ErrClosed
	}
	C.DS_FreeStream(s.state)
	s.state = nil
	s.model.removeStream(s)
	return nil
}

func (s *stream) IntermediateDecode() string {
	cstr := C.DS_IntermediateDecode(s.state)
	defer C.DS_FreeString(cstr)
	result := C.GoString(cstr)
	return result
}

func (s *stream) FeedAudioContent(frame []int16) {
	if len(frame) == 0 {
		return
	}
	C.DS_FeedAudioContent(
		s.state,
		(*C.short)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&frame)).Data)),
		C.uint(len(frame)))
}

func (s *stream) IntermediateDecodeWithMetadata(aNumResults uint32) *deepspeech.Metadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	return toMetadata(C.DS_IntermediateDecodeWithMetadata(s.state, C.uint32_t(aNumResults)))
}

func (s *stream) FinishStream() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return ""
	}
	result := C.DS_FinishStream(s.state)
	defer C.DS_FreeString(result)
	res := C.GoString(result)
	s.state = nil
	s.model.removeStream(s)
	return res
}

// Signal the end of an audio signal to an ongoing streaming
//        inference, returns per-letter metadata.
//
// @param aSctx A streaming state pointer returned by {@link DS_CreateStream()}.
//
// @return Outputs a struct of individual letters along with their timing information.
//         The user is responsible for freeing Metadata by calling {@link DS_FreeMetadata()}. Returns NULL on error.
//
// @note This method will free the state pointer (@p aSctx).
func (s *stream) FinishStreamWithMetadata(aNumResults uint32) *deepspeech.Metadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	mt := toMetadata(C.DS_FinishStreamWithMetadata(s.state, C.uint32_t(aNumResults)))
	s.state = nil
	s.model.removeStream(s)
	return mt
}

func (s *stream) FinishStreamWithBestHypothesis(aNumResults uint32) *deepspeech.HypothesisCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	mt := toMetadata(C.DS_FinishStreamWithMetadata(s.state, C.uint32_t(aNumResults)))
	s.state = nil
	s.model.removeStream(s)

	hyp := NewHypothesis(mt)
	if len(hyp.Candidates) == 0 {
		return nil
	}
	best := hyp.Candidates[0]
	return &best
}

func (s *stream) FinishStreamWithHypothesis(aNumResults uint32) deepspeech.Hypothesis {
	s.mu.Lock()
	defer s.mu.Unlock()
	mt := toMetadata(C.DS_FinishStreamWithMetadata(s.state, C.uint32_t(aNumResults)))
	s.state = nil
	s.model.removeStream(s)
	return NewHypothesis(mt)
}

func toMetadata(cMt *C.struct_Metadata) *deepspeech.Metadata {
	mt := (*metadata)(unsafe.Pointer(cMt))
	transcripts := ((*[1 << 30]candidateTranscript)(mt.transcripts))[:mt.num_transcripts:mt.num_transcripts]
	t := make([]deepspeech.CandidateTranscript, len(transcripts))
	for i, transcript := range transcripts {
		tokens := ((*[1 << 30]tokenMetadata)(transcript.tokens))[:transcript.num_tokens:transcript.num_tokens]
		ct := deepspeech.CandidateTranscript{
			Tokens:     make([]deepspeech.TokenMetadata, int(transcript.num_tokens)),
			Confidence: float64(transcript.confidence),
		}
		t[i] = ct

		for d, token := range tokens {
			ct.Tokens[d] = deepspeech.TokenMetadata{
				Text:      C.GoString(token.text),
				Timestep:  int(token.timestep),
				StartTime: float32(token.start_time),
			}
		}
	}

	// Free Metadata memory
	C.DS_FreeMetadata(cMt)

	return &deepspeech.Metadata{
		Transcripts: t,
	}
}

func NewHypothesis(m *deepspeech.Metadata) deepspeech.Hypothesis {
	candidates := make([]deepspeech.HypothesisCandidate, len(m.Transcripts))
	for i, c := range m.Transcripts {
		candidates[i] = newHypothesisCandidate(&c)
	}
	return deepspeech.Hypothesis{
		Candidates: candidates,
	}
}

func newHypothesisCandidate(m *deepspeech.CandidateTranscript) deepspeech.HypothesisCandidate {
	isWhitespace := true
	startStep := 0
	startTime := time.Duration(0)
	hyp := deepspeech.HypothesisCandidate{
		Confidence: m.Confidence,
		Words:      make([]deepspeech.Word, 0, 8),
	}

	items := m.Tokens
	text := strings.Builder{}
	wordBuf := strings.Builder{}

	lastIndex := len(items) - 1
	for i, item := range items {
		if i == 0 {
			startStep = item.Timestep
			startTime = time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime)))
		}

		c := item.Text

		text.WriteString(c)

		// Trim whitespace.
		c = strings.TrimSpace(c)

		// Is it whitespace?
		if len(c) == 0 {
			// End of word?
			if !isWhitespace {
				isWhitespace = true
				// Only if not first character.
				if i > 0 {
					endStep := item.Timestep
					endTime := time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime)))
					w := deepspeech.Word{
						Whitespace: false,
						Value:      wordBuf.String(),
						StartStep:  startStep,
						EndStep:    endStep,
						StartTime:  startTime,
						EndTime:    endTime,
					}
					w.Duration = w.EndTime - w.StartTime
					hyp.Words = append(hyp.Words, w)

					w.StartStep = w.EndStep + 1
					w.StartTime = w.EndTime

					wordBuf.Reset()
				} else if i == lastIndex {
					//w := Word{
					//	Whitespace: false,
					//	Value:      wordBuf.String(),
					//	StartStep:  startStep,
					//	EndStep:    item.Timestep(),
					//	StartTime:  startTime,
					//	EndTime:    time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime()))),
					//}
					//w.Duration = w.EndTime - w.StartTime
					//hyp.Words = append(hyp.Words, w)
				}

				startStep = item.Timestep
				startTime = time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime)))
			} else if i == lastIndex {
				//w := Word{
				//	Whitespace: true,
				//	Value:      "",
				//	StartStep:  startStep,
				//	EndStep:    item.Timestep,
				//	StartTime:  startTime,
				//	EndTime:    time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime))),
				//}
				//w.Duration = w.EndTime - w.StartTime
				//hyp.Words = append(hyp.Words, w)
			}
		} else {
			wordBuf.WriteString(c)

			// End of whitespace?
			if isWhitespace {

				isWhitespace = false
				// Only if not first character.
				if i > 0 {
					//w := Word{
					//	Whitespace: true,
					//	Value:      "",
					//	StartStep:  startStep,
					//	EndStep:    item.Timestep,
					//	StartTime:  startTime,
					//	EndTime:    time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime))),
					//}
					//w.Duration = w.EndTime - w.StartTime
					//hyp.Words = append(hyp.Words, w)
					//startStep = w.EndStep
					//startTime = w.EndTime

					startStep = item.Timestep
					startTime = time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime)))
				} else if i == lastIndex {
					w := deepspeech.Word{
						Whitespace: false,
						Value:      wordBuf.String(),
						StartStep:  startStep,
						EndStep:    item.Timestep,
						StartTime:  startTime,
						EndTime:    time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime))),
					}
					w.Duration = w.EndTime - w.StartTime
					hyp.Words = append(hyp.Words, w)
				}

				startStep = item.Timestep
				startTime = time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime)))
			} else if i == lastIndex {
				w := deepspeech.Word{
					Whitespace: false,
					Value:      wordBuf.String(),
					StartStep:  startStep,
					EndStep:    item.Timestep,
					StartTime:  startTime,
					EndTime:    time.Duration(math.RoundToEven(float64(time.Second) * float64(item.StartTime))),
				}
				w.Duration = w.EndTime - w.StartTime
				hyp.Words = append(hyp.Words, w)
			}
		}
	}

	hyp.Text = text.String()
	if len(hyp.Words) > 0 {
		first := hyp.Words[0]
		last := hyp.Words[len(hyp.Words)-1]
		hyp.StartStep = first.StartStep
		hyp.StartTime = first.StartTime
		hyp.EndStep = last.EndStep
		hyp.EndTime = last.EndTime
		hyp.Duration = hyp.EndTime - hyp.StartTime
	}

	return hyp
}
