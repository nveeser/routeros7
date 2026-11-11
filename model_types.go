package routerosv7

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type MList []string

func (b MList) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.Join(b, ","))
}

func (b *MList) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = strings.Split(v, ",")
	return nil
}

type MInt int

func (b MInt) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.Itoa(int(b)) + `"`), nil
}

func (b *MInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == `""` || raw == "null" {
		*b = 0
		return nil
	}
	// Strip surrounding quotes if present
	raw = strings.Trim(raw, `"`)
	val, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}
	*b = MInt(val)
	return nil
}

type Mbool bool

// MarshalJSON serializes the bool as a quoted JSON string for RouterOS REST: "true" or "false".
func (b Mbool) MarshalJSON() ([]byte, error) {
	if b {
		return []byte(`"true"`), nil
	}
	return []byte(`"false"`), nil
}

// UnmarshalJSON parses RouterOS responses whether returned as strings ("true", "false", "yes", "no")
// or native JSON booleans (true, false).
func (b *Mbool) UnmarshalJSON(data []byte) error {
	if v, ok := toBool(string(data)); ok {
		*b = v
		return nil
	}
	// Handle quoted string values from RouterOS
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("Mbool: failed to unmarshal %s: %w", string(data), err)
	}
	if v, ok := toBool(s); ok {
		*b = v
		return nil
	}
	return fmt.Errorf("Mbool: unhandled boolean string value %q", s)
}

func toBool(s string) (Mbool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "enabled":
		return true, true
	case "false", "no", "0", "disabled", "":
		return false, true
	default:
		return false, false
	}
}
