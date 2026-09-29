package files

import "encoding/json"

// PostgreSQL drivers may materialize TIMESTAMPTZ with the process offset.
// R1 wire timestamps always use UTC without changing the represented instant.
func (d Descriptor) MarshalJSON() ([]byte, error) {
	type wire Descriptor
	v := wire(d)
	v.CreatedAt = v.CreatedAt.UTC()
	v.UpdatedAt = v.UpdatedAt.UTC()
	if v.ExpiresAt != nil {
		at := v.ExpiresAt.UTC()
		v.ExpiresAt = &at
	}
	if v.DeletedAt != nil {
		at := v.DeletedAt.UTC()
		v.DeletedAt = &at
	}
	return json.Marshal(v)
}
