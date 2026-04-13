package wasmloader

import (
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/generators"
	"github.com/gopxl/beep/v2/speaker"
)

const audioSampleRate = 48000

var (
	speakerOnce sync.Once
	speakerErr  error
)

func initSpeaker() error {
	speakerOnce.Do(func() {
		sr := beep.SampleRate(audioSampleRate)
		speakerErr = speaker.Init(sr, sr.N(time.Second/10))
	})
	return speakerErr
}

// playChime synthesizes and plays a chime sound with the given parameters.
// freq:         fundamental frequency in Hz (e.g. 200–1300)
// decay:        decay rate (2.0 = long ring, 12.0 = short plonk)
// volume:       amplitude (0.0–1.0)
// richness:     harmonic mix (0.0 = pure sine, 0.8 = rich harmonics)
// harmonicMult: frequency multiplier for the harmonic (e.g. 1.5–4.0)
func playChime(freq, decay, volume, richness, harmonicMult float64) {
	if err := initSpeaker(); err != nil {
		return
	}

	sr := beep.SampleRate(audioSampleRate)

	// Duration: higher decay = shorter sound
	seconds := 2.0 / decay
	duration := time.Duration(seconds * float64(time.Second))
	numSamples := sr.N(duration)

	// Fundamental sine tone
	fundamental, err := generators.SineTone(sr, freq)
	if err != nil {
		return
	}

	var s beep.Streamer = fundamental

	// Mix in a harmonic for timbral richness
	if richness > 0.05 {
		harmonic, err := generators.SineTone(sr, freq*harmonicMult)
		if err == nil {
			s = beep.Mix(
				&effects.Gain{Streamer: fundamental, Gain: -richness},
				&effects.Gain{Streamer: harmonic, Gain: richness - 1},
			)
		}
	}

	// Apply volume (linear gain)
	s = &effects.Gain{Streamer: s, Gain: volume - 1}

	// Fade out over the full duration
	s = effects.Transition(s, numSamples, 1.0, 0.0, effects.TransitionEqualPower)

	// Limit to the computed duration
	s = beep.Take(numSamples, s)

	// Non-blocking playback
	speaker.Play(s)
}
