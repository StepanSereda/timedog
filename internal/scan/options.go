package scan

// Options mirror timedog flags.
type Options struct {
	Depth         *int   `json:"depth,omitempty"`          // -d: summarize below this segment depth
	OmitSymlinks  bool   `json:"omit_symlinks"`            // -l
	MinSizeBytes  *int64 `json:"min_size_bytes,omitempty"` // -m
	SortBy        int    `json:"sort_by"`                  // 0=old 1=new 2=name (timedog -S)
	UseBase10     bool   `json:"use_base10"`               // -H
	SimpleFormat  bool   `json:"simple_format"`            // -n (affects display strings only)
	// FastWalk uses parallel directory traversal (github.com/charlievieth/fastwalk).
	// If nil, defaults to true. Callback state is mutex-protected; order is normalized by sort at the end.
	FastWalk *bool `json:"fast_walk,omitempty"`
}

func (o Options) fastWalkEnabled() bool {
	if o.FastWalk == nil {
		return true
	}
	return *o.FastWalk
}
