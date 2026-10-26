//go:build !windows

package audio

func Inputs() []Device  { return nil }
func Outputs() []Device { return nil }

// Record has no backend off Windows.
func Record(id, rate, chunkMs int, fn func(pcm []float32)) (Recorder, error) {
	return nil, ErrUnsupported
}

// NewPlayer has no backend off Windows.
func NewPlayer(id, rate int) (Player, error) { return nil, ErrUnsupported }
