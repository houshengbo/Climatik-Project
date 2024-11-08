package algorithms

type DynamicFrequencyScaler struct {
	ScalingFactor float64
	MinFrequency  int32
}

func NewDynamicFrequencyScaler(scalingFactor float64, minFreq int32) *DynamicFrequencyScaler {
	return &DynamicFrequencyScaler{
		ScalingFactor: scalingFactor,
		MinFrequency:  minFreq,
	}
}

func (d *DynamicFrequencyScaler) CalculateNewFrequency(currentFreq int32) int32 {
	newFreq := int32(float64(currentFreq) * d.ScalingFactor)
	if newFreq < d.MinFrequency {
		return d.MinFrequency
	}
	return newFreq
}
