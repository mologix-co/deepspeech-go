package deepspeech

import (
	"github.com/mologix-co/deepspeech-go/model"
	"plugin"
)

const (
	BeamWidth = model.BeamWidth
	LMAlpha   = model.LMAlpha
	LMBeta    = model.LMBeta
)

var (
	lib      *plugin.Plugin
	newModel func(string, uint32) (model.Model, error)
	version  string
)

type Config struct {
	BeamWidth uint32
	LMAlpha   float32
	LMBeta    float32
}

type Stream model.Stream
type Model model.Model
type TokenMetadata model.TokenMetadata
type CandidateTranscript model.CandidateTranscript
type Metadata model.Metadata
type Hypothesis model.Hypothesis
type HypothesisCandidate model.HypothesisCandidate
type Word model.Word

func Version() string {
	return version
}

func init() {
	load()

	// Extract.
	var err error
	lib, err = plugin.Open("deepspeech_plugin.so")
	if err != nil {
		panic(err)
	}

	symbol, err := lib.Lookup("New")
	if err != nil {
		panic(err)
	}

	var ok bool
	newModel, ok = symbol.(func(string, uint32) (model.Model, error))
	if !ok {
		panic("deepspeech.New does not match signature func(string, uint32) (Model, error)")
	}

	symbol, err = lib.Lookup("Version")
	if err != nil {
		panic(err)
	}
	versionFn, ok := symbol.(func() string)
	if !ok {
		panic("deepspeech.Version does not match signature func() string")
	}
	version = versionFn()
	model.SetVersion(version)
}

func Open(modelPath, scorerPath string, config Config) (model.Model, error) {
	if config.BeamWidth <= 0 {
		config.BeamWidth = BeamWidth
	}
	if config.LMAlpha <= 0 {
		config.LMAlpha = LMAlpha
	}
	if config.LMBeta <= 0 {
		config.LMBeta = LMBeta
	}

	m, err := newModel(modelPath, config.BeamWidth)
	if err != nil {
		return nil, err
	}
	err = m.EnableExternalScorer(scorerPath, config.LMAlpha, config.LMBeta)
	if err != nil {
		_ = m.Close()
		return nil, err
	}
	return m, nil
}

func DefaultConfig() Config {
	return Config{
		BeamWidth: BeamWidth,
		LMAlpha:   LMAlpha,
		LMBeta:    LMBeta,
	}
}
