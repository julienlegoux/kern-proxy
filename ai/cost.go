package ai

// Ports: packages/ai/src/models.ts (calculateCost, getSupportedThinkingLevels,
// clampThinkingLevel)

// CalculateCost fills usage.Cost from the model's price sheet and returns it.
//
// Anthropic charges 2x base input for 1h cache writes: the CacheWrite1h subset
// of CacheWrite is billed at 2*cost.Input instead of cost.CacheWrite.
func CalculateCost(model *Model, usage *Usage) UsageCost {
	longWrite := 0
	if usage.CacheWrite1h != nil {
		longWrite = *usage.CacheWrite1h
	}
	shortWrite := usage.CacheWrite - longWrite
	usage.Cost.Input = model.Cost.Input / 1e6 * float64(usage.Input)
	usage.Cost.Output = model.Cost.Output / 1e6 * float64(usage.Output)
	usage.Cost.CacheRead = model.Cost.CacheRead / 1e6 * float64(usage.CacheRead)
	usage.Cost.CacheWrite = (model.Cost.CacheWrite*float64(shortWrite) + model.Cost.Input*2*float64(longWrite)) / 1e6
	usage.Cost.Total = usage.Cost.Input + usage.Cost.Output + usage.Cost.CacheRead + usage.Cost.CacheWrite
	return usage.Cost
}

// extendedThinkingLevels is the ordered ladder of thinking levels.
var extendedThinkingLevels = []ModelThinkingLevel{
	ThinkingOff, ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingXHigh,
}

// GetSupportedThinkingLevels lists the thinking levels the model supports.
// Non-reasoning models support only "off". A ThinkingLevelMap entry of nil
// (TS null) marks a level unsupported; "xhigh" additionally requires an
// explicit map entry.
func GetSupportedThinkingLevels(model *Model) []ModelThinkingLevel {
	if !model.Reasoning {
		return []ModelThinkingLevel{ThinkingOff}
	}
	var out []ModelThinkingLevel
	for _, level := range extendedThinkingLevels {
		mapped, present := model.ThinkingLevelMap[level]
		if present && mapped == nil {
			continue
		}
		if level == ThinkingXHigh && !present {
			continue
		}
		out = append(out, level)
	}
	return out
}

// ClampThinkingLevel clamps level to the nearest supported one: exact match
// first, then upward along the ladder, then downward, then the first
// available level ("off" as a last resort).
func ClampThinkingLevel(model *Model, level ModelThinkingLevel) ModelThinkingLevel {
	available := GetSupportedThinkingLevels(model)
	contains := func(l ModelThinkingLevel) bool {
		for _, a := range available {
			if a == l {
				return true
			}
		}
		return false
	}
	if contains(level) {
		return level
	}
	requested := -1
	for i, l := range extendedThinkingLevels {
		if l == level {
			requested = i
			break
		}
	}
	if requested == -1 {
		if len(available) > 0 {
			return available[0]
		}
		return ThinkingOff
	}
	for i := requested; i < len(extendedThinkingLevels); i++ {
		if contains(extendedThinkingLevels[i]) {
			return extendedThinkingLevels[i]
		}
	}
	for i := requested - 1; i >= 0; i-- {
		if contains(extendedThinkingLevels[i]) {
			return extendedThinkingLevels[i]
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return ThinkingOff
}
