package llamaswap

import "time"

// DemoSample returns only deterministic synthetic aliases and coarse states.
func DemoSample(at time.Time) Sample {
	sample, err := NewAvailableSample(at, at, ModeDemo, []Model{
		{ID: "example-chat", State: StateLoaded},
		{ID: "example-embed", State: StateUnloaded},
	})
	if err != nil {
		panic(err)
	}
	return sample
}
