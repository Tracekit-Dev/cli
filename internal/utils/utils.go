package utils

// MaskAPIKey masks an API key for display
func MaskAPIKey(apiKey string) string {
	if len(apiKey) < 20 {
		return apiKey
	}
	return apiKey[:15] + "..." + apiKey[len(apiKey)-4:]
}
