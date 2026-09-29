package domain

import ai "InkLore/internal/llm/domain"

func EstimateTokenCountForModel(text string, modelName string) int {
	return ai.EstimateTextTokens(text, modelName)
}

func EstimateTokenCount(text string) int {
	return EstimateTokenCountForModel(text, "")
}
