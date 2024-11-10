package algorithms

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PowerProfile struct {
	Frequency    int32
	AveragePower float64
	StdDev      float64
	P95Power    float64
	P99Power    float64
}

func (p PowerProfile) getPowerForScalingFactor(factor float64) float64 {
	switch {
	case factor >= 0.99:
		return p.P99Power
	case factor >= 0.95:
		return p.P95Power
	default:
		return p.AveragePower
	}
}

type DynamicFrequencyScaler struct {
	ScalingFactor float64
	MinFrequency  int32
	PowerCap      float64
	powerProfile  []PowerProfile
}

func loadPowerProfile(filepath string) ([]PowerProfile, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Skip header
	_, err = reader.Read()
	if err != nil {
		return nil, err
	}

	var profile []PowerProfile
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		freq, err := strconv.ParseInt(record[0], 10, 32)
		if err != nil {
			return nil, err
		}

		avgPower, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return nil, err
		}

		stdDev, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, err
		}

		p95Power, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return nil, err
		}

		p99Power, err := strconv.ParseFloat(record[4], 64)
		if err != nil {
			return nil, err
		}

		profile = append(profile, PowerProfile{
			Frequency:    int32(freq),
			AveragePower: avgPower,
			StdDev:      stdDev,
			P95Power:    p95Power,
			P99Power:    p99Power,
		})
	}

	return profile, nil
}

func NewDynamicFrequencyScaler(ctx context.Context, scalingFactor float64, minFreq int32, powerCap float64) *DynamicFrequencyScaler {
	profilePath := os.Getenv("POWER_PROFILE_PATH")
	if profilePath == "" {
		profilePath = "config/power_profiles/inf-V10016GB-profile.csv"
	}

	profile, err := loadPowerProfile(profilePath)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to load power profile")
		return nil
	}

	return &DynamicFrequencyScaler{
		ScalingFactor: scalingFactor,
		MinFrequency:  minFreq,
		PowerCap:      powerCap,
		powerProfile:  profile,
	}
}

func (d *DynamicFrequencyScaler) findFrequencyForPower(ctx context.Context, targetPower float64) int32 {
	for _, entry := range d.powerProfile {
		powerMetric := entry.getPowerForScalingFactor(d.ScalingFactor)
		log.FromContext(ctx).Info("Checking power profile entry", 
			"frequency", entry.Frequency, 
			"powerMetric", powerMetric,
			"scalingFactor", d.ScalingFactor)

		if powerMetric <= targetPower {
			return entry.Frequency
		}
	}
	return d.MinFrequency
}

// CalculateFrequency calculates the target frequency for a given power budget
func (d *DynamicFrequencyScaler) CalculateFrequency(ctx context.Context, powerBudget float64) int32 {
	targetFreq := d.findFrequencyForPower(ctx, powerBudget)
	log.FromContext(ctx).Info("Calculated target frequency", "powerBudget", powerBudget, "targetFreq", targetFreq)
	return targetFreq
}
