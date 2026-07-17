package dm

// SanitizedPlayer is the condensed, DM-facing view of a character. With the
// server-authoritative store, Summary carries the one-line capability digest
// built by game.CapabilityDigest.
type SanitizedPlayer struct {
	ID        string
	Name      string
	ClassName string
	Subclass  string
	Summary   string
}

func arrTake(arr []any, n int) []any {
	if n < len(arr) {
		return arr[:n]
	}
	return arr
}

func firstNonEmpty(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
