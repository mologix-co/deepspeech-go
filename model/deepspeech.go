package model

import (
	"errors"
	"time"
)

const (
	BeamWidth = 500
	LMAlpha   = 0.931289039105002
	LMBeta    = 1.1834137581510284
)

type Model interface {
	EnableExternalScorer(path string, aAlpha, aBeta float32) error

	SampleRate() int

	Close() error

	SpeechToText(frame []int16) string

	CreateStream() (Stream, error)
}

type Stream interface {
	Free() error

	IntermediateDecode() string

	FeedAudioContent(frame []int16)

	IntermediateDecodeWithMetadata(aNumResults uint32) *Metadata

	FinishStream() string

	FinishStreamWithMetadata(aNumResults uint32) *Metadata

	FinishStreamWithBestHypothesis(aNumResults uint32) *HypothesisCandidate

	FinishStreamWithHypothesis(aNumResults uint32) Hypothesis
}

type TokenMetadata struct {
	Text      string
	Timestep  int
	StartTime float32
}

type CandidateTranscript struct {
	Tokens     []TokenMetadata
	Confidence float64
}

type Metadata struct {
	Transcripts []CandidateTranscript
}

type Hypothesis struct {
	Candidates []HypothesisCandidate
}

type HypothesisCandidate struct {
	Text       string
	Confidence float64
	StartStep  int
	EndStep    int
	StartTime  time.Duration
	EndTime    time.Duration
	Duration   time.Duration
	Words      []Word
}

type Word struct {
	Whitespace bool
	Value      string
	StartStep  int
	EndStep    int
	StartTime  time.Duration
	EndTime    time.Duration
	Duration   time.Duration
}

var (
	// Missing model
	ErrNoModel = errNoModel(0x1000)

	// Invalid parameters
	ErrInvalidAlphabet       = errInvalidAlphabet(0x2000)
	ErrInvalidShape          = errInvalidShape(0x2001)
	ErrInvalidScorer         = errInvalidScorer(0x2002)
	ErrModelIncompatible     = errModelIncompatible(0x2003)
	ErrScorerNotEnabled      = errScorerNotEnabled(0x2004)
	ErrScorerUnreadable      = errScorerUnreadable(0x2005)
	ErrScorerInvalidLM       = errScorerInvalidLM(0x2006)
	ErrScorerNoTrie          = errScorerNoTrie(0x2007)
	ErrScorerInvalidTrie     = errScorerInvalidTrie(0x2008)
	ErrScorerVersionMismatch = errScorerVersionMismatch(0x2009)

	// Runtime failures
	ErrFailInitMMAP     = errFailInitMMAP(0x3000)
	ErrFailInitSess     = errFailInitSess(0x3001)
	ErrFailInterpreter  = errFailInterpreter(0x3002)
	ErrFailRunSess      = errFailRunSess(0x3003)
	ErrFailCreateStream = errFailCreateStream(0x3004)
	ErrFailReadProtobuf = errFailReadProtobuf(0x3005)
	ErrFailCreateSess   = errFailCreateSess(0x3006)
	ErrFailCreateModel  = errFailCreateModel(0x3007)

	ErrOpenStreams = errors.New("open streams")

	version = ""
)

func Version() string {
	return version
}

func SetVersion(v string) {
	version = v
}

func ErrorOf(code int) error {
	if code == 0 {
		return nil
	}
	switch code {
	case 0x1000:
		return ErrNoModel
	case 0x2000:
		return ErrInvalidAlphabet
	case 0x2001:
		return ErrInvalidShape
	case 0x2002:
		return ErrInvalidScorer
	case 0x2003:
		return ErrModelIncompatible
	case 0x2004:
		return ErrScorerNotEnabled
	case 0x2005:
		return ErrScorerUnreadable
	case 0x2006:
		return ErrScorerInvalidLM
	case 0x2007:
		return ErrScorerNoTrie
	case 0x2008:
		return ErrScorerInvalidTrie
	case 0x2009:
		return ErrScorerVersionMismatch

	case 0x3000:
		return ErrFailInitMMAP
	case 0x3001:
		return ErrFailInitSess
	case 0x3002:
		return ErrFailInterpreter
	case 0x3003:
		return ErrFailRunSess
	case 0x3004:
		return ErrFailCreateStream
	case 0x3005:
		return ErrFailReadProtobuf
	case 0x3006:
		return ErrFailCreateSess
	case 0x3007:
		return ErrFailCreateModel
	}
	return Error(code)
}

type Error int

func (e Error) Error() string {
	err := ErrorOf(int(e))
	if err != e {
		return err.Error()
	} else {
		return "unknown"
	}
}

type errNoModel Error

func (errNoModel) Error() string { return "Missing model information." }

type errInvalidAlphabet Error

func (errInvalidAlphabet) Error() string {
	return "Invalid alphabet embedded in model. (Data corruption?)"
}

type errInvalidShape Error

func (errInvalidShape) Error() string { return "Invalid model shape." }

type errInvalidScorer Error

func (errInvalidScorer) Error() string { return "Invalid scorer file." }

type errModelIncompatible Error

func (errModelIncompatible) Error() string { return "Incompatible model." }

type errScorerNotEnabled Error

func (errScorerNotEnabled) Error() string { return "External scorer is not enabled." }

type errScorerUnreadable Error

func (errScorerUnreadable) Error() string { return "Could not read scorer file." }

type errScorerInvalidLM Error

func (errScorerInvalidLM) Error() string {
	return "Could not recognize language model header in scorer."
}

type errScorerNoTrie Error

func (errScorerNoTrie) Error() string {
	return "Reached end of scorer file before loading vocabulary trie."
}

type errScorerInvalidTrie Error

func (errScorerInvalidTrie) Error() string { return "Invalid magic in trie header." }

type errScorerVersionMismatch Error

func (errScorerVersionMismatch) Error() string {
	return "Scorer file version does not match expected version."
}

type errFailInitMMAP Error

func (errFailInitMMAP) Error() string { return "Failed to initialize memory mapped model." }

type errFailInitSess Error

func (errFailInitSess) Error() string { return "Failed to initialize the session." }

type errFailInterpreter Error

func (errFailInterpreter) Error() string { return "Interpreter failed." }

type errFailRunSess Error

func (errFailRunSess) Error() string { return "Failed to run the session." }

type errFailCreateStream Error

func (errFailCreateStream) Error() string { return "Error creating the stream." }

type errFailReadProtobuf Error

func (errFailReadProtobuf) Error() string { return "Error reading the proto buffer model file." }

type errFailCreateSess Error

func (errFailCreateSess) Error() string { return "Failed to create session." }

type errFailCreateModel Error

func (errFailCreateModel) Error() string { return "Could not allocate model state." }
