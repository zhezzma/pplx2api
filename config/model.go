package config

var ModelReverseMap = map[string]string{}
var ModelMap = map[string]string{
	"claude-3.7-sonnet":       "claude2",
	"claude-3.7-sonnet-think": "claude37sonnetthinking",
	"deepseek-r1":             "r1",
	"gpt-4.5":                 "gpt45",
	"o3-mini":                 "o3mini",
	"gpt-4o":                  "gpt4o",
	"gemini-2.0-flash":        "gemini2flash",
	"grok-2":                  "grok",
}

// Get returns the value for the given key from the ModelMap.
// If the key doesn't exist, it returns the provided default value.
func ModelMapGet(key string, defaultValue string) string {
	if value, exists := ModelMap[key]; exists {
		return value
	}
	return defaultValue
}

// GetReverse returns the value for the given key from the ModelReverseMap.
// If the key doesn't exist, it returns the provided default value.
func ModelReverseMapGet(key string, defaultValue string) string {
	if value, exists := ModelReverseMap[key]; exists {
		return value
	}
	return defaultValue
}

var ResponseModles []map[string]string

func init() {
	for k, v := range ModelMap {
		ModelReverseMap[v] = k
		model := map[string]string{
			"id": k,
		}
		modelSearch := map[string]string{
			"id": k + "-search",
		}
		ResponseModles = append(ResponseModles, model, modelSearch)
	}
}
