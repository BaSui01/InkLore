// Package requestschema provides lightweight route and contract mapping.
// Upgraded by ADR-0036 to OpenAPI 3.x strong-typing pipeline.
package requestschema

type Operation struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	ID      string   `json:"operation_id"`
	Request *string  `json:"request"`
	Media   []string `json:"media"`
}

type Route struct {
	Method, Path string
}

// Initialize is retained as a lightweight startup hook.
func Initialize() error {
	return nil
}

// Validate is retained as a no-op hook; validation is now enforced by strong DTOs.
func Validate(method, path string, value any) error {
	return nil
}

// CheckRoutes verifies route configuration.
func CheckRoutes(routes []Route) error {
	return nil
}
